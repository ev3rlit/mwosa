package dailybar

import (
	"context"
	"fmt"

	provider "github.com/ev3rlit/mwosa/providers/core"
	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage"
	entdb "github.com/ev3rlit/mwosa/storage/ent"
	dailybarent "github.com/ev3rlit/mwosa/storage/ent/dailybar"
)

type readRepository struct {
	database *storage.Database
}

var _ daily.ReadRepository = (*readRepository)(nil)

func NewReadRepository(databasePath string) daily.ReadRepository {
	return &readRepository{database: storage.NewDatabase(databasePath)}
}

func NewRepositories(databasePath string) (daily.ReadRepository, daily.WriteRepository) {
	database := storage.NewDatabase(databasePath)
	return &readRepository{database: database}, &writeRepository{database: database}
}

func (r *readRepository) QueryDailyBars(ctx context.Context, query daily.Query) ([]coredailybar.Bar, error) {
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

	bars := make([]coredailybar.Bar, 0, len(rows))
	for _, row := range rows {
		bar, err := entDailyBarToCanonical(row)
		if err != nil {
			return nil, err
		}
		bars = append(bars, bar)
	}
	return bars, nil
}

func (r *readRepository) open(ctx context.Context) (*entdb.Client, error) {
	if r == nil || r.database == nil {
		return nil, fmt.Errorf("daily bar read repository database is nil")
	}
	return r.database.Open(ctx)
}
