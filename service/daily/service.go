package daily

import (
	"context"
	"fmt"
	"time"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
)

type Store interface {
	UpsertDailyBars(ctx context.Context, bars []dailybar.Bar) (WriteResult, error)
	QueryDailyBars(ctx context.Context, query Query) ([]dailybar.Bar, error)
}

type Query struct {
	Market       provider.Market
	SecurityType provider.SecurityType
	Symbol       string
	From         string
	To           string
}

type WriteResult struct {
	FilesWritten int
	BarsWritten  int
}

type Service struct {
	Router dailybar.Router
	Store  Store
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
	FilesWritten int                   `json:"files_written"`
}

func (s Service) Get(ctx context.Context, req Request) (BarsResult, error) {
	if s.Store == nil {
		return BarsResult{}, fmt.Errorf("daily service store is nil")
	}
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return BarsResult{}, err
	}
	query := queryFromRequest(req, dates)
	bars, err := s.Store.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, err
	}
	if len(bars) == 0 {
		return BarsResult{}, notFound(req, query)
	}
	return BarsResult{Bars: bars}, nil
}

func (s Service) Ensure(ctx context.Context, req Request) (BarsResult, error) {
	if req.Symbol == "" {
		return BarsResult{}, fmt.Errorf("ensure daily requires symbol")
	}
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return BarsResult{}, err
	}
	if len(dates) == 0 {
		return BarsResult{}, fmt.Errorf("ensure daily requires --as-of or --from/--to")
	}

	query := queryFromRequest(req, dates)
	existing, err := s.Store.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, err
	}
	missingDates := datesWithoutSymbol(existing, dates, req.Symbol)
	for _, date := range missingDates {
		if _, err := s.collectDate(ctx, req, date); err != nil {
			return BarsResult{}, err
		}
	}

	bars, err := s.Store.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, err
	}
	if len(bars) == 0 {
		return BarsResult{}, notFound(req, query)
	}
	return BarsResult{Bars: bars}, nil
}

func (s Service) Sync(ctx context.Context, req Request) (CollectResult, error) {
	date, err := parseDate(req.AsOf, "--as-of")
	if err != nil {
		return CollectResult{}, err
	}
	return s.collectDate(ctx, req, date)
}

func (s Service) Backfill(ctx context.Context, req Request) (CollectResult, error) {
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return CollectResult{}, err
	}
	if len(dates) == 0 {
		return CollectResult{}, fmt.Errorf("backfill daily requires --from/--to")
	}

	result := CollectResult{
		Market:       withDefaultMarket(req.Market),
		SecurityType: req.SecurityType,
		Dates:        make([]string, 0, len(dates)),
	}
	for _, date := range dates {
		partial, err := s.collectDate(ctx, req, date)
		if err != nil {
			return CollectResult{}, err
		}
		result.ProviderID = partial.ProviderID
		result.Group = partial.Group
		result.Dates = append(result.Dates, partial.Dates...)
		result.BarsFetched += partial.BarsFetched
		result.BarsStored += partial.BarsStored
		result.FilesWritten += partial.FilesWritten
	}
	return result, nil
}

func (s Service) collectDate(ctx context.Context, req Request, date time.Time) (CollectResult, error) {
	if s.Router == nil {
		return CollectResult{}, fmt.Errorf("daily service router is nil")
	}
	if s.Store == nil {
		return CollectResult{}, fmt.Errorf("daily service store is nil")
	}
	if req.SecurityType == "" {
		return CollectResult{}, fmt.Errorf("daily collection requires --security-type")
	}

	market := withDefaultMarket(req.Market)
	fetcher, err := s.Router.RouteDailyBars(ctx, dailybar.RouteInput{
		ProviderID:     req.ProviderID,
		PreferProvider: req.PreferProvider,
		Market:         market,
		SecurityType:   req.SecurityType,
		Symbol:         req.Symbol,
	})
	if err != nil {
		return CollectResult{}, err
	}

	result, err := fetcher.FetchDailyBars(ctx, dailybar.FetchInput{
		Market:       market,
		SecurityType: req.SecurityType,
		From:         apiDate(date),
		To:           apiDate(date),
	})
	if err != nil {
		return CollectResult{}, err
	}
	writeResult, err := s.Store.UpsertDailyBars(ctx, result.Bars)
	if err != nil {
		return CollectResult{}, err
	}

	return CollectResult{
		Market:       market,
		SecurityType: req.SecurityType,
		ProviderID:   result.Provider.ID,
		Group:        result.Group,
		Dates:        []string{isoDate(date)},
		BarsFetched:  len(result.Bars),
		BarsStored:   writeResult.BarsWritten,
		FilesWritten: writeResult.FilesWritten,
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
	return &NotFoundError{
		Symbol:       req.Symbol,
		Market:       query.Market,
		SecurityType: query.SecurityType,
		From:         query.From,
		To:           query.To,
		Hint:         hint,
	}
}
