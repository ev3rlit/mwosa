package dailybar

import (
	"context"
	"time"

	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage"
	entdb "github.com/ev3rlit/mwosa/storage/ent"
	dailybarent "github.com/ev3rlit/mwosa/storage/ent/dailybar"
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

	tx, err := client.Tx(ctx)
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
		err = tx.DailyBar.Create().
			SetProvider(string(bar.Provider)).
			SetProviderGroup(string(bar.Group)).
			SetOperation(string(bar.Operation)).
			SetMarket(string(bar.Market)).
			SetSecurityType(string(bar.SecurityType)).
			SetSymbol(bar.Symbol).
			SetIsin(bar.ISIN).
			SetName(bar.Name).
			SetTradingDate(bar.TradingDate).
			SetCurrency(bar.Currency).
			SetOpeningPrice(bar.Open).
			SetHighestPrice(bar.High).
			SetLowestPrice(bar.Low).
			SetClosingPrice(bar.Close).
			SetPriceChangeFromPreviousClose(bar.Change).
			SetPriceChangeRateFromPreviousClose(bar.ChangeRate).
			SetTradedVolume(bar.Volume).
			SetTradedAmount(bar.TradedValue).
			SetMarketCapitalization(bar.MarketCap).
			SetExtensionsJSON(extensionsJSON).
			OnConflictColumns(
				dailybarent.FieldMarket,
				dailybarent.FieldSecurityType,
				dailybarent.FieldTradingDate,
				dailybarent.FieldSymbol,
				dailybarent.FieldProvider,
				dailybarent.FieldProviderGroup,
			).
			Update(func(upsert *entdb.DailyBarUpsert) {
				upsert.UpdateOperation()
				upsert.UpdateIsin()
				upsert.UpdateName()
				upsert.UpdateCurrency()
				upsert.UpdateOpeningPrice()
				upsert.UpdateHighestPrice()
				upsert.UpdateLowestPrice()
				upsert.UpdateClosingPrice()
				upsert.UpdatePriceChangeFromPreviousClose()
				upsert.UpdatePriceChangeRateFromPreviousClose()
				upsert.UpdateTradedVolume()
				upsert.UpdateTradedAmount()
				upsert.UpdateMarketCapitalization()
				upsert.UpdateExtensionsJSON()
				upsert.SetUpdatedAt(now)
			}).
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
