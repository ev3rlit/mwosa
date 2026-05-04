package dailybar

import (
	"context"
	"time"

	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage"
	"github.com/samber/oops"
)

type writeRepository struct {
	database *storage.Database
}

var _ daily.WriteRepository = (*writeRepository)(nil)

func NewWriteRepository(database *storage.Database) (daily.WriteRepository, error) {
	if database == nil {
		return nil, oops.In("dailybar_repository").New("daily bar repository database is nil")
	}
	return &writeRepository{database: database}, nil
}

func (r *writeRepository) UpsertDailyBars(ctx context.Context, bars []coredailybar.Bar) (daily.WriteResult, error) {
	errb := oops.In("dailybar_repository").With("bars", len(bars))

	client, err := r.database.Client(ctx)
	if err != nil {
		return daily.WriteResult{}, errb.Wrap(err)
	}

	tx, err := client.BeginTx(ctx, nil)
	if err != nil {
		return daily.WriteResult{}, errb.Wrapf(err, "begin daily bar sqlite transaction")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, bar := range bars {
		barErrb := errb.With(
			"provider", bar.Provider,
			"group", bar.Group,
			"market", bar.Market,
			"security_type", bar.SecurityType,
			"date", bar.TradingDate,
			"symbol", bar.Symbol,
		)

		if err := validateBarKey(bar); err != nil {
			return daily.WriteResult{}, barErrb.Wrap(err)
		}
		extensionsJSON, err := encodeExtensions(bar.Extensions)
		if err != nil {
			return daily.WriteResult{}, barErrb.Wrap(err)
		}
		now := time.Now()
		row := dailyBarToRow(bar, extensionsJSON, now)
		_, err = tx.NewInsert().
			Model(&row).
			On("CONFLICT (market, security_type, trading_date, symbol, provider, provider_group) DO UPDATE").
			Set("operation = EXCLUDED.operation").
			Set("isin = EXCLUDED.isin").
			Set("name = EXCLUDED.name").
			Set("currency = EXCLUDED.currency").
			Set("opening_price = EXCLUDED.opening_price").
			Set("highest_price = EXCLUDED.highest_price").
			Set("lowest_price = EXCLUDED.lowest_price").
			Set("closing_price = EXCLUDED.closing_price").
			Set("price_change_from_previous_close = EXCLUDED.price_change_from_previous_close").
			Set("price_change_rate_from_previous_close = EXCLUDED.price_change_rate_from_previous_close").
			Set("traded_volume = EXCLUDED.traded_volume").
			Set("traded_amount = EXCLUDED.traded_amount").
			Set("market_capitalization = EXCLUDED.market_capitalization").
			Set("extensions_json = EXCLUDED.extensions_json").
			Set("updated_at = EXCLUDED.updated_at").
			Exec(ctx)
		if err != nil {
			return daily.WriteResult{}, barErrb.Wrapf(err, "upsert daily bar sqlite row")
		}
	}

	if err := tx.Commit(); err != nil {
		return daily.WriteResult{}, errb.Wrapf(err, "commit daily bar sqlite transaction")
	}
	committed = true

	return daily.WriteResult{
		BarsWritten:  len(bars),
		RowsAffected: len(bars),
	}, nil
}

func dailyBarToRow(bar coredailybar.Bar, extensionsJSON string, now time.Time) storage.DailyBarRow {
	return storage.DailyBarRow{
		Provider:                         string(bar.Provider),
		ProviderGroup:                    string(bar.Group),
		Operation:                        string(bar.Operation),
		Market:                           string(bar.Market),
		SecurityType:                     string(bar.SecurityType),
		Symbol:                           bar.Symbol,
		ISIN:                             bar.ISIN,
		Name:                             bar.Name,
		TradingDate:                      bar.TradingDate,
		Currency:                         bar.Currency,
		OpeningPrice:                     bar.Open,
		HighestPrice:                     bar.High,
		LowestPrice:                      bar.Low,
		ClosingPrice:                     bar.Close,
		PriceChangeFromPreviousClose:     bar.Change,
		PriceChangeRateFromPreviousClose: bar.ChangeRate,
		TradedVolume:                     bar.Volume,
		TradedAmount:                     bar.TradedValue,
		MarketCapitalization:             bar.MarketCap,
		ExtensionsJSON:                   extensionsJSON,
		CreatedAt:                        now,
		UpdatedAt:                        now,
	}
}
