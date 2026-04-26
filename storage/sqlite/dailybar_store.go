package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	_ "modernc.org/sqlite"
)

type DailyBarStore struct {
	databasePath string
}

func NewDailyBarStore(databasePath string) *DailyBarStore {
	return &DailyBarStore{databasePath: databasePath}
}

func (s *DailyBarStore) UpsertDailyBars(ctx context.Context, bars []dailybar.Bar) (daily.WriteResult, error) {
	db, err := s.open(ctx)
	if err != nil {
		return daily.WriteResult{}, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return daily.WriteResult{}, fmt.Errorf("begin daily bar sqlite transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, upsertDailyBarSQL)
	if err != nil {
		return daily.WriteResult{}, fmt.Errorf("prepare daily bar sqlite upsert: %w", err)
	}
	defer stmt.Close()

	result := daily.WriteResult{BarsWritten: len(bars)}
	for _, bar := range bars {
		if err := validateBarKey(bar); err != nil {
			return daily.WriteResult{}, err
		}
		extensionsJSON, err := encodeExtensions(bar.Extensions)
		if err != nil {
			return daily.WriteResult{}, err
		}
		execResult, err := stmt.ExecContext(
			ctx,
			string(bar.Provider),
			string(bar.Group),
			string(bar.Operation),
			string(bar.Market),
			string(bar.SecurityType),
			bar.Symbol,
			bar.ISIN,
			bar.Name,
			bar.TradingDate,
			bar.Currency,
			bar.Open,
			bar.High,
			bar.Low,
			bar.Close,
			bar.Change,
			bar.ChangeRate,
			bar.Volume,
			bar.TradedValue,
			bar.MarketCap,
			extensionsJSON,
		)
		if err != nil {
			return daily.WriteResult{}, fmt.Errorf("upsert daily bar sqlite row market=%s security_type=%s date=%s symbol=%s provider=%s group=%s: %w", bar.Market, bar.SecurityType, bar.TradingDate, bar.Symbol, bar.Provider, bar.Group, err)
		}
		if rowsAffected, err := execResult.RowsAffected(); err == nil {
			result.RowsAffected += int(rowsAffected)
		}
	}

	if err := tx.Commit(); err != nil {
		return daily.WriteResult{}, fmt.Errorf("commit daily bar sqlite transaction: %w", err)
	}
	return result, nil
}

func (s *DailyBarStore) QueryDailyBars(ctx context.Context, query daily.Query) ([]dailybar.Bar, error) {
	db, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	sqlText, args := buildDailyBarQuery(query)
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("query daily bars sqlite market=%s security_type=%s symbol=%s from=%s to=%s: %w", query.Market, query.SecurityType, query.Symbol, query.From, query.To, err)
	}
	defer rows.Close()

	bars := make([]dailybar.Bar, 0)
	for rows.Next() {
		var bar dailybar.Bar
		var extensionsJSON string
		if err := rows.Scan(
			&bar.Provider,
			&bar.Group,
			&bar.Operation,
			&bar.Market,
			&bar.SecurityType,
			&bar.Symbol,
			&bar.ISIN,
			&bar.Name,
			&bar.TradingDate,
			&bar.Currency,
			&bar.Open,
			&bar.High,
			&bar.Low,
			&bar.Close,
			&bar.Change,
			&bar.ChangeRate,
			&bar.Volume,
			&bar.TradedValue,
			&bar.MarketCap,
			&extensionsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan daily bar sqlite row: %w", err)
		}
		extensions, err := decodeExtensions(extensionsJSON)
		if err != nil {
			return nil, err
		}
		bar.Extensions = extensions
		bars = append(bars, bar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily bar sqlite rows: %w", err)
	}
	return bars, nil
}

func (s *DailyBarStore) open(ctx context.Context) (*sql.DB, error) {
	if s == nil || strings.TrimSpace(s.databasePath) == "" {
		return nil, fmt.Errorf("daily bar sqlite database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite database directory: %w", err)
	}
	db, err := sql.Open("sqlite", s.databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %s: %w", s.databasePath, err)
	}
	if err := setupSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func setupSchema(ctx context.Context, db *sql.DB) error {
	for _, statement := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
		createDailyBarTableSQL,
		createDailyBarDateIndexSQL,
		createDailyBarSymbolIndexSQL,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("setup sqlite daily bar schema: %w", err)
		}
	}
	return nil
}

const createDailyBarTableSQL = `
CREATE TABLE IF NOT EXISTS daily_bar (
	provider TEXT NOT NULL,
	provider_group TEXT NOT NULL,
	operation TEXT NOT NULL,
	market TEXT NOT NULL,
	security_type TEXT NOT NULL,
	symbol TEXT NOT NULL,
	isin TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL DEFAULT '',
	trading_date TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT '',
	opening_price TEXT NOT NULL DEFAULT '',
	highest_price TEXT NOT NULL DEFAULT '',
	lowest_price TEXT NOT NULL DEFAULT '',
	closing_price TEXT NOT NULL DEFAULT '',
	price_change_from_previous_close TEXT NOT NULL DEFAULT '',
	price_change_rate_from_previous_close TEXT NOT NULL DEFAULT '',
	traded_volume TEXT NOT NULL DEFAULT '',
	traded_amount TEXT NOT NULL DEFAULT '',
	market_capitalization TEXT NOT NULL DEFAULT '',
	extensions_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
	PRIMARY KEY (market, security_type, trading_date, symbol, provider, provider_group)
)`

const createDailyBarDateIndexSQL = `
CREATE INDEX IF NOT EXISTS idx_daily_bar_date
ON daily_bar (market, security_type, trading_date)`

const createDailyBarSymbolIndexSQL = `
CREATE INDEX IF NOT EXISTS idx_daily_bar_symbol_date
ON daily_bar (market, security_type, symbol, trading_date)`

const upsertDailyBarSQL = `
INSERT INTO daily_bar (
	provider,
	provider_group,
	operation,
	market,
	security_type,
	symbol,
	isin,
	name,
	trading_date,
	currency,
	opening_price,
	highest_price,
	lowest_price,
	closing_price,
	price_change_from_previous_close,
	price_change_rate_from_previous_close,
	traded_volume,
	traded_amount,
	market_capitalization,
	extensions_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (market, security_type, trading_date, symbol, provider, provider_group) DO UPDATE SET
	operation = excluded.operation,
	isin = excluded.isin,
	name = excluded.name,
	currency = excluded.currency,
	opening_price = excluded.opening_price,
	highest_price = excluded.highest_price,
	lowest_price = excluded.lowest_price,
	closing_price = excluded.closing_price,
	price_change_from_previous_close = excluded.price_change_from_previous_close,
	price_change_rate_from_previous_close = excluded.price_change_rate_from_previous_close,
	traded_volume = excluded.traded_volume,
	traded_amount = excluded.traded_amount,
	market_capitalization = excluded.market_capitalization,
	extensions_json = excluded.extensions_json,
	updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`

func buildDailyBarQuery(query daily.Query) (string, []any) {
	market := query.Market
	if market == "" {
		market = provider.MarketKRX
	}

	conditions := []string{"market = ?"}
	args := []any{string(market)}
	if query.SecurityType != "" {
		conditions = append(conditions, "security_type = ?")
		args = append(args, string(query.SecurityType))
	}
	if query.Symbol != "" {
		conditions = append(conditions, "symbol = ?")
		args = append(args, query.Symbol)
	}
	if query.From != "" {
		conditions = append(conditions, "trading_date >= ?")
		args = append(args, query.From)
	}
	if query.To != "" {
		conditions = append(conditions, "trading_date <= ?")
		args = append(args, query.To)
	}

	return fmt.Sprintf(`
SELECT
	provider,
	provider_group,
	operation,
	market,
	security_type,
	symbol,
	isin,
	name,
	trading_date,
	currency,
	opening_price,
	highest_price,
	lowest_price,
	closing_price,
	price_change_from_previous_close,
	price_change_rate_from_previous_close,
	traded_volume,
	traded_amount,
	market_capitalization,
	extensions_json
FROM daily_bar
WHERE %s
ORDER BY trading_date ASC, symbol ASC, provider ASC, provider_group ASC`, strings.Join(conditions, " AND ")), args
}

func validateBarKey(bar dailybar.Bar) error {
	if bar.Market == "" || bar.SecurityType == "" || bar.TradingDate == "" || bar.Symbol == "" || bar.Provider == "" || bar.Group == "" {
		return fmt.Errorf("daily bar missing sqlite key provider=%s group=%s market=%s security_type=%s date=%s symbol=%s", bar.Provider, bar.Group, bar.Market, bar.SecurityType, bar.TradingDate, bar.Symbol)
	}
	return nil
}

func encodeExtensions(extensions map[string]string) (string, error) {
	if len(extensions) == 0 {
		return "{}", nil
	}
	bytes, err := json.Marshal(extensions)
	if err != nil {
		return "", fmt.Errorf("encode daily bar extensions: %w", err)
	}
	return string(bytes), nil
}

func decodeExtensions(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return nil, nil
	}
	var extensions map[string]string
	if err := json.Unmarshal([]byte(raw), &extensions); err != nil {
		return nil, fmt.Errorf("decode daily bar extensions: %w", err)
	}
	return extensions, nil
}
