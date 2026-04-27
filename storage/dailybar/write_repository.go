package dailybar

import (
	"context"
	"fmt"
	"time"

	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage"
	entdb "github.com/ev3rlit/mwosa/storage/ent"
	dailybarent "github.com/ev3rlit/mwosa/storage/ent/dailybar"
)

type writeRepository struct {
	database *storage.Database
}

var _ daily.WriteRepository = (*writeRepository)(nil)

func NewWriteRepository(database *storage.Database) daily.WriteRepository {
	return &writeRepository{database: database}
}

func (r *writeRepository) UpsertDailyBars(ctx context.Context, bars []coredailybar.Bar) (daily.WriteResult, error) {
	client, err := r.client(ctx)
	if err != nil {
		return daily.WriteResult{}, err
	}

	tx, err := client.Tx(ctx)
	if err != nil {
		return daily.WriteResult{}, fmt.Errorf("begin daily bar sqlite transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, bar := range bars {
		if err := validateBarKey(bar); err != nil {
			return daily.WriteResult{}, err
		}
		extensionsJSON, err := encodeExtensions(bar.Extensions)
		if err != nil {
			return daily.WriteResult{}, err
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
			return daily.WriteResult{}, fmt.Errorf("upsert daily bar sqlite row market=%s security_type=%s date=%s symbol=%s provider=%s group=%s: %w", bar.Market, bar.SecurityType, bar.TradingDate, bar.Symbol, bar.Provider, bar.Group, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return daily.WriteResult{}, fmt.Errorf("commit daily bar sqlite transaction: %w", err)
	}
	committed = true

	return daily.WriteResult{
		BarsWritten:  len(bars),
		RowsAffected: len(bars),
	}, nil
}

func (r *writeRepository) client(ctx context.Context) (*entdb.Client, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("daily bar write repository database is nil")
	}
	return r.database.Client(ctx)
}
