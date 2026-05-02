package daily

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/samber/oops"
)

const (
	defaultBackfillWorkers = 1
	maxBackfillWorkers     = 16
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

type ReadService struct {
	reader ReadRepository
}

type Service struct {
	router dailybar.Router
	reader ReadRepository
	writer WriteRepository
}

func NewReadService(reader ReadRepository) (ReadService, error) {
	if reader == nil {
		return ReadService{}, oops.In("daily_service").New("daily service read repository is nil")
	}
	return ReadService{reader: reader}, nil
}

func NewService(reader ReadRepository, writer WriteRepository, router dailybar.Router) (Service, error) {
	errb := oops.In("daily_service")
	if reader == nil {
		return Service{}, errb.New("daily service read repository is nil")
	}
	if writer == nil {
		return Service{}, errb.New("daily service write repository is nil")
	}
	if router == nil {
		return Service{}, errb.New("daily service router is nil")
	}
	return Service{
		router: router,
		reader: reader,
		writer: writer,
	}, nil
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
	Workers        int
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

	bars []dailybar.Bar
}

func (s ReadService) Get(ctx context.Context, req Request) (BarsResult, error) {
	reqErrb := oops.In("daily_service").With("symbol", req.Symbol, "from", req.From, "to", req.To, "as_of", req.AsOf)
	if s.reader == nil {
		return BarsResult{}, reqErrb.New("daily service read repository is nil")
	}
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return BarsResult{}, reqErrb.Wrap(err)
	}
	query := queryFromRequest(req, dates)
	queryErrb := oops.In("daily_service").With("market", query.Market, "security_type", query.SecurityType, "symbol", query.Symbol, "from", query.From, "to", query.To)
	bars, err := s.reader.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, queryErrb.Wrapf(err, "get daily bars")
	}
	if len(bars) == 0 {
		return BarsResult{}, notFound(req, query)
	}
	return BarsResult{Bars: bars}, nil
}

func (s Service) Get(ctx context.Context, req Request) (BarsResult, error) {
	return ReadService{reader: s.reader}.Get(ctx, req)
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
	if s.reader == nil {
		return BarsResult{}, queryErrb.New("daily service read repository is nil")
	}
	existing, err := s.reader.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, queryErrb.Wrapf(err, "query existing daily bars")
	}
	missingDates := datesWithoutSymbol(existing, dates, req.Symbol)
	collectedBars := make([]dailybar.Bar, 0)
	for _, date := range missingDates {
		result, err := s.collectDate(ctx, req, date)
		if err != nil {
			return BarsResult{}, reqErrb.With("market", withDefaultMarket(req.Market), "security_type", req.SecurityType, "date", apiDate(date)).Wrapf(err, "collect missing daily bars")
		}
		collectedBars = append(collectedBars, result.bars...)
	}

	bars, err := s.reader.QueryDailyBars(ctx, query)
	if err != nil {
		return BarsResult{}, queryErrb.Wrapf(err, "query stored daily bars")
	}
	if len(bars) == 0 {
		if resolvedSymbol := singleCollectedSymbol(collectedBars); resolvedSymbol != "" && resolvedSymbol != req.Symbol {
			resolvedQuery := query
			resolvedQuery.Symbol = resolvedSymbol
			bars, err = s.reader.QueryDailyBars(ctx, resolvedQuery)
			if err != nil {
				return BarsResult{}, queryErrb.With("resolved_symbol", resolvedSymbol).Wrapf(err, "query stored daily bars by resolved symbol")
			}
		}
	}
	if len(bars) == 0 && len(collectedBars) > 0 {
		return BarsResult{Bars: collectedBars}, nil
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
	errb := oops.In("daily_service").With("from", req.From, "to", req.To, "as_of", req.AsOf, "workers", req.Workers)
	dates, err := resolveDateRange(req.From, req.To, req.AsOf)
	if err != nil {
		return CollectResult{}, errb.Wrap(err)
	}
	if len(dates) == 0 {
		return CollectResult{}, errb.New("backfill daily requires --from/--to")
	}
	workers, err := normalizeBackfillWorkers(req.Workers)
	if err != nil {
		return CollectResult{}, errb.Wrap(err)
	}
	if workers > 1 && len(dates) > 1 {
		return s.backfillWithWorkers(ctx, req, dates, workers)
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
		mergeCollectResult(&result, partial)
	}
	return result, nil
}

func (s Service) collectDate(ctx context.Context, req Request, date time.Time) (CollectResult, error) {
	result, err := s.fetchDate(ctx, req, date)
	if err != nil {
		return CollectResult{}, err
	}
	return s.storeCollection(ctx, result)
}

type dateJob struct {
	index int
	date  time.Time
}

type dateFetchResult struct {
	index  int
	result CollectResult
	err    error
}

func (s Service) backfillWithWorkers(ctx context.Context, req Request, dates []time.Time, workers int) (CollectResult, error) {
	errb := oops.In("daily_service").With("from", req.From, "to", req.To, "workers", workers)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan dateJob)
	results := make(chan dateFetchResult)
	var wg sync.WaitGroup
	for workerID := 0; workerID < workers; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				result, err := s.fetchDate(workerCtx, req, job.date)
				select {
				case results <- dateFetchResult{index: job.index, result: result, err: err}:
				case <-workerCtx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for index, date := range dates {
			select {
			case jobs <- dateJob{index: index, date: date}:
			case <-workerCtx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	result := CollectResult{
		Market:       withDefaultMarket(req.Market),
		SecurityType: req.SecurityType,
		Dates:        make([]string, 0, len(dates)),
	}
	var firstErr error
	for fetched := range results {
		if fetched.err != nil {
			if firstErr == nil {
				firstErr = errb.With("date", apiDate(dates[fetched.index])).Wrapf(fetched.err, "backfill daily worker fetch")
				cancel()
			}
			continue
		}
		if firstErr != nil {
			continue
		}
		partial, err := s.storeCollection(ctx, fetched.result)
		if err != nil {
			if firstErr == nil {
				firstErr = errb.With("date", fetched.result.Dates).Wrapf(err, "backfill daily worker store")
				cancel()
			}
			continue
		}
		mergeCollectResult(&result, partial)
	}
	if firstErr != nil {
		return CollectResult{}, firstErr
	}
	sort.Strings(result.Dates)
	return result, nil
}

func (s Service) fetchDate(ctx context.Context, req Request, date time.Time) (CollectResult, error) {
	market := withDefaultMarket(req.Market)
	dateText := apiDate(date)
	errb := oops.In("daily_service").With("market", market, "security_type", req.SecurityType, "date", dateText)

	if s.router == nil {
		return CollectResult{}, errb.New("daily service router is nil")
	}
	if req.SecurityType == "" {
		return CollectResult{}, errb.New("daily collection requires --security-type")
	}

	fetcher, err := s.router.RouteDailyBars(ctx, dailybar.RouteInput{
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
		Symbol:       req.Symbol,
		From:         dateText,
		To:           dateText,
	})
	if err != nil {
		return CollectResult{}, errb.With("provider", req.ProviderID).Wrapf(err, "fetch daily bars")
	}

	return CollectResult{
		Market:       market,
		SecurityType: req.SecurityType,
		ProviderID:   result.Provider.ID,
		Group:        result.Group,
		Dates:        []string{isoDate(date)},
		BarsFetched:  len(result.Bars),
		bars:         result.Bars,
	}, nil
}

func (s Service) storeCollection(ctx context.Context, result CollectResult) (CollectResult, error) {
	errb := oops.In("daily_service").With("provider", result.ProviderID, "group", result.Group, "bars", len(result.bars))
	if s.writer == nil {
		return CollectResult{}, errb.New("daily service write repository is nil")
	}
	writeResult, err := s.writer.UpsertDailyBars(ctx, result.bars)
	if err != nil {
		return CollectResult{}, errb.Wrapf(err, "store daily bars")
	}
	result.BarsStored = writeResult.BarsWritten
	result.RowsAffected = writeResult.RowsAffected
	return result, nil
}

func normalizeBackfillWorkers(workers int) (int, error) {
	if workers == 0 {
		return defaultBackfillWorkers, nil
	}
	if workers < 0 {
		return 0, oops.In("daily_service").With("workers", workers).Errorf("workers must be positive: %d", workers)
	}
	if workers > maxBackfillWorkers {
		return 0, oops.In("daily_service").With("workers", workers, "max_workers", maxBackfillWorkers).Errorf("workers must be <= %d: %d", maxBackfillWorkers, workers)
	}
	return workers, nil
}

func mergeCollectResult(result *CollectResult, partial CollectResult) {
	result.ProviderID = partial.ProviderID
	result.Group = partial.Group
	result.Dates = append(result.Dates, partial.Dates...)
	result.BarsFetched += partial.BarsFetched
	result.BarsStored += partial.BarsStored
	result.RowsAffected += partial.RowsAffected
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

func singleCollectedSymbol(bars []dailybar.Bar) string {
	symbol := ""
	for _, bar := range bars {
		if bar.Symbol == "" {
			continue
		}
		if symbol == "" {
			symbol = bar.Symbol
			continue
		}
		if bar.Symbol != symbol {
			return ""
		}
	}
	return symbol
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
