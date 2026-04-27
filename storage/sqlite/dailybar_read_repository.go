package sqlite

import (
	"context"
	"fmt"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	entdb "github.com/ev3rlit/mwosa/storage/sqlite/ent"
	dailybarent "github.com/ev3rlit/mwosa/storage/sqlite/ent/dailybar"
)

type DailyBarReadRepository struct {
	database *Database
}

func NewDailyBarReadRepository(databasePath string) *DailyBarReadRepository {
	return &DailyBarReadRepository{database: NewDatabase(databasePath)}
}

func NewDailyBarRepositories(databasePath string) (*DailyBarReadRepository, *DailyBarWriteRepository) {
	database := NewDatabase(databasePath)
	return &DailyBarReadRepository{database: database}, &DailyBarWriteRepository{database: database}
}

func (r *DailyBarReadRepository) QueryDailyBars(ctx context.Context, query daily.Query) ([]dailybar.Bar, error) {
	client, err := r.open(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	market := query.Market
	if market == "" {
		market = provider.MarketKRX
	}

	builder := client.DailyBar.Query().
		Where(dailybarent.MarketEQ(string(market))).
		Order(
			entdb.Asc(dailybarent.FieldTradingDate),
			entdb.Asc(dailybarent.FieldSymbol),
			entdb.Asc(dailybarent.FieldProvider),
			entdb.Asc(dailybarent.FieldProviderGroup),
		)

	if query.SecurityType != "" {
		builder.Where(dailybarent.SecurityTypeEQ(string(query.SecurityType)))
	}
	if query.Symbol != "" {
		builder.Where(dailybarent.SymbolEQ(query.Symbol))
	}
	if query.From != "" {
		builder.Where(dailybarent.TradingDateGTE(query.From))
	}
	if query.To != "" {
		builder.Where(dailybarent.TradingDateLTE(query.To))
	}

	rows, err := builder.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query daily bars sqlite market=%s security_type=%s symbol=%s from=%s to=%s: %w", query.Market, query.SecurityType, query.Symbol, query.From, query.To, err)
	}

	bars := make([]dailybar.Bar, 0, len(rows))
	for _, row := range rows {
		bar, err := entDailyBarToCanonical(row)
		if err != nil {
			return nil, err
		}
		bars = append(bars, bar)
	}
	return bars, nil
}

func (r *DailyBarReadRepository) open(ctx context.Context) (*entdb.Client, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("daily bar read repository database is nil")
	}
	return r.database.open(ctx)
}
