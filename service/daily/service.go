package daily

import (
	"context"
	"fmt"
	"time"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/samber/oops"
)

type ReadRepository interface {
	QueryDailyBars(ctx context.Context, query Query) ([]dailybar.Bar, error)
}

type WriteRepository interface {
	UpsertDailyBars(ctx context.Context, bars []dailybar.Bar) (WriteResult, error)
}

type Query struct {
	Market       provider.Market
	SecurityType provider.SecurityType
	Symbol       string
	From         string
	To           string
}

type WriteResult struct {
	RowsAffected int
	BarsWritten  int
}

type Service struct {
	Router dailybar.Router
	Reader ReadRepository
	Writer WriteRepository
}

type Request struct {
	ProviderID     provider.ProviderID
	PreferProvider provider.ProviderID
	Market         provider.Market
	SecurityType   provider.SecurityType
	Symbol         string
	From           string
	To             string
	AsOf           string
}

type BarsResult struct {
	Bars []dailybar.Bar
}

type CollectResult struct {
	Market       provider.Market       `json:"market"`
	SecurityType provider.SecurityType `json:"security_type"`
	ProviderID   provider.ProviderID   `json:"provider"`
	Group        provider.GroupID      `json:"provider_group"`
	Dates        []string              `json:"dates"`
	BarsFetched  int                   `json:"bars_fetched"`
	BarsStored   int                   `json:"bars_stored"`
	RowsAffected int                   `json:"rows_affected"`
}

func (s Service) Get(ctx context.Context, req Request) (BarsResult, error) {
	reqErrb := oops.In("daily_service").With("symbol", req.Symbol, "from", req.From, "to", req.To, "as_of", req.AsOf)
	if s.Reader == nil {
		return BarsResult{}, reqErrb.New("daily service read repository is nil")
	}
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return BarsResult{}, reqErrb.Wrap(err)
	}
	query := queryFromRequest(req, dates)
	queryErrb := oops.In("daily_service").With("market", query.Market, "security_type", query.SecurityType, "symbol", query.Symbol, "from", query.From, "to", query.To)
	bars, err := s.Reader.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, queryErrb.Wrapf(err, "get daily bars")
	}
	if len(bars) == 0 {
		return BarsResult{}, notFound(req, query)
	}
	return BarsResult{Bars: bars}, nil
}

func (s Service) Ensure(ctx context.Context, req Request) (BarsResult, error) {
	reqErrb := oops.In("daily_service").With("symbol", req.Symbol, "from", req.From, "to", req.To, "as_of", req.AsOf)
	if req.Symbol == "" {
		return BarsResult{}, reqErrb.New("ensure daily requires symbol")
	}
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return BarsResult{}, reqErrb.Wrap(err)
	}
	if len(dates) == 0 {
		return BarsResult{}, reqErrb.New("ensure daily requires --as-of or --from/--to")
	}

	query := queryFromRequest(req, dates)
	queryErrb := oops.In("daily_service").With("market", query.Market, "security_type", query.SecurityType, "symbol", query.Symbol, "from", query.From, "to", query.To)
	if s.Reader == nil {
		return BarsResult{}, queryErrb.New("daily service read repository is nil")
	}
	existing, err := s.Reader.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, queryErrb.Wrapf(err, "query existing daily bars")
	}
	missingDates := datesWithoutSymbol(existing, dates, req.Symbol)
	for _, date := range missingDates {
		if _, err := s.collectDate(ctx, req, date); err != nil {
			return BarsResult{}, reqErrb.With("market", withDefaultMarket(req.Market), "security_type", req.SecurityType, "date", apiDate(date)).Wrapf(err, "collect missing daily bars")
		}
	}

	bars, err := s.Reader.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, queryErrb.Wrapf(err, "query stored daily bars")
	}
	if len(bars) == 0 {
		return BarsResult{}, notFound(req, query)
	}
	return BarsResult{Bars: bars}, nil
}

func (s Service) Sync(ctx context.Context, req Request) (CollectResult, error) {
	date, err := parseDate(req.AsOf, "--as-of")
	if err != nil {
		return CollectResult{}, oops.In("daily_service").With("as_of", req.AsOf).Wrap(err)
	}
	return s.collectDate(ctx, req, date)
}

func (s Service) Backfill(ctx context.Context, req Request) (CollectResult, error) {
	errb := oops.In("daily_service").With("from", req.From, "to", req.To, "as_of", req.AsOf)
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return CollectResult{}, errb.Wrap(err)
	}
	if len(dates) == 0 {
		return CollectResult{}, errb.New("backfill daily requires --from/--to")
	}

	result := CollectResult{
		Market:       withDefaultMarket(req.Market),
		SecurityType: req.SecurityType,
		Dates:        make([]string, 0, len(dates)),
	}
	for _, date := range dates {
		partial, err := s.collectDate(ctx, req, date)
		if err != nil {
			return CollectResult{}, errb.With("market", withDefaultMarket(req.Market), "security_type", req.SecurityType, "date", apiDate(date)).Wrapf(err, "backfill daily date")
		}
		result.ProviderID = partial.ProviderID
		result.Group = partial.Group
		result.Dates = append(result.Dates, partial.Dates...)
		result.BarsFetched += partial.BarsFetched
		result.BarsStored += partial.BarsStored
		result.RowsAffected += partial.RowsAffected
	}
	return result, nil
}

func (s Service) collectDate(ctx context.Context, req Request, date time.Time) (CollectResult, error) {
	market := withDefaultMarket(req.Market)
	dateText := apiDate(date)
	errb := oops.In("daily_service").With("market", market, "security_type", req.SecurityType, "date", dateText)

	if s.Router == nil {
		return CollectResult{}, errb.New("daily service router is nil")
	}
	if s.Writer == nil {
		return CollectResult{}, errb.New("daily service write repository is nil")
	}
	if req.SecurityType == "" {
		return CollectResult{}, errb.New("daily collection requires --security-type")
	}

	fetcher, err := s.Router.RouteDailyBars(ctx, dailybar.RouteInput{
		ProviderID:     req.ProviderID,
		PreferProvider: req.PreferProvider,
		Market:         market,
		SecurityType:   req.SecurityType,
		Symbol:         req.Symbol,
	})
	if err != nil {
		return CollectResult{}, errb.With("provider", req.ProviderID, "prefer_provider", req.PreferProvider, "symbol", req.Symbol).Wrapf(err, "route daily bars")
	}

	result, err := fetcher.FetchDailyBars(ctx, dailybar.FetchInput{
		Market:       market,
		SecurityType: req.SecurityType,
		From:         dateText,
		To:           dateText,
	})
	if err != nil {
		return CollectResult{}, errb.With("provider", req.ProviderID).Wrapf(err, "fetch daily bars")
	}
	writeResult, err := s.Writer.UpsertDailyBars(ctx, result.Bars)
	if err != nil {
		return CollectResult{}, errb.With("provider", result.Provider.ID, "group", result.Group, "bars", len(result.Bars)).Wrapf(err, "store daily bars")
	}

	return CollectResult{
		Market:       market,
		SecurityType: req.SecurityType,
		ProviderID:   result.Provider.ID,
		Group:        result.Group,
		Dates:        []string{isoDate(date)},
		BarsFetched:  len(result.Bars),
		BarsStored:   writeResult.BarsWritten,
		RowsAffected: writeResult.RowsAffected,
	}, nil
}

func queryFromRequest(req Request, dates []time.Time) Query {
	query := Query{
		Market:       withDefaultMarket(req.Market),
		SecurityType: req.SecurityType,
		Symbol:       req.Symbol,
	}
	if len(dates) > 0 {
		query.From = isoDate(dates[0])
		query.To = isoDate(dates[len(dates)-1])
	}
	return query
}

func withDefaultMarket(market provider.Market) provider.Market {
	if market == "" {
		return provider.MarketKRX
	}
	return market
}

func datesWithoutSymbol(existing []dailybar.Bar, dates []time.Time, symbol string) []time.Time {
	found := make(map[string]bool, len(existing))
	for _, bar := range existing {
		if bar.Symbol == symbol {
			found[bar.TradingDate] = true
		}
	}
	missing := make([]time.Time, 0)
	for _, date := range dates {
		if !found[isoDate(date)] {
			missing = append(missing, date)
		}
	}
	return missing
}

func notFound(req Request, query Query) error {
	hint := fmt.Sprintf("run `mwosa ensure daily %s --from %s --to %s`", req.Symbol, query.From, query.To)
	if query.From == "" && query.To == "" {
		hint = fmt.Sprintf("run `mwosa ensure daily %s --as-of <YYYYMMDD>`", req.Symbol)
	}
	return oops.In("daily_service").With(
		"market", query.Market,
		"security_type", query.SecurityType,
		"symbol", req.Symbol,
		"from", query.From,
		"to", query.To,
	).Wrap(&NotFoundError{
		Symbol:       req.Symbol,
		Market:       query.Market,
		SecurityType: query.SecurityType,
		From:         query.From,
		To:           query.To,
		Hint:         hint,
	})
}
