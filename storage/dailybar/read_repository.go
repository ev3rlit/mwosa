package dailybar

import (
	"context"

	provider "github.com/ev3rlit/mwosa/providers/core"
	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage"
	entdb "github.com/ev3rlit/mwosa/storage/ent"
	dailybarent "github.com/ev3rlit/mwosa/storage/ent/dailybar"
	"github.com/samber/oops"
)

type readRepository struct {
	database *storage.Database
}

var _ daily.ReadRepository = (*readRepository)(nil)

func NewReadRepository(database *storage.Database) (daily.ReadRepository, error) {
	if database == nil {
		return nil, oops.In("dailybar_repository").New("daily bar repository database is nil")
	}
	return &readRepository{database: database}, nil
}

func NewRepositories(database *storage.Database) (daily.ReadRepository, daily.WriteRepository, error) {
	if database == nil {
		return nil, nil, oops.In("dailybar_repository").New("daily bar repository database is nil")
	}
	return &readRepository{database: database}, &writeRepository{database: database}, nil
}

func (r *readRepository) QueryDailyBars(ctx context.Context, query daily.Query) ([]coredailybar.Bar, error) {
	errb := oops.In("dailybar_repository").With(
		"market", query.Market,
		"security_type", query.SecurityType,
		"symbol", query.Symbol,
		"from", query.From,
		"to", query.To,
	)

	client, err := r.database.Client(ctx)
	if err != nil {
		return nil, errb.Wrap(err)
	}

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
		return nil, errb.Wrapf(err, "query daily bars sqlite")
	}

	bars := make([]coredailybar.Bar, 0, len(rows))
	for _, row := range rows {
		bar, err := entDailyBarToCanonical(row)
		if err != nil {
			return nil, errb.With("row_id", row.ID).Wrap(err)
		}
		bars = append(bars, bar)
	}
	return bars, nil
}
