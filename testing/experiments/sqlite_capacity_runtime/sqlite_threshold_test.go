package sqlite_capacity_runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteThresholdProbeConfigJSON = `
{
  "enabled": true,
  "output_dir": "tmp/testing/sqlite-capacity-runtime/threshold",
  "clean_output_dir": true,
  "symbols": 1100,
  "trading_days": [7, 30, 120, 250, 750],
  "query_runs": 20,
  "top_n": 100,
  "timeout_seconds": 180
}
`

type sqliteThresholdProbeConfig struct {
	Enabled        bool   `json:"enabled"`
	OutputDir      string `json:"output_dir"`
	CleanOutputDir bool   `json:"clean_output_dir"`
	Symbols        int    `json:"symbols"`
	TradingDays    []int  `json:"trading_days"`
	QueryRuns      int    `json:"query_runs"`
	TopN           int    `json:"top_n"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type sqliteThresholdResult struct {
	TradingDays int
	Rows        int
	FileBytes   int64
	OpenP50     time.Duration
	CountP50    time.Duration
	RankP50     time.Duration
}

func TestSQLiteThresholdProbe(t *testing.T) {
	config := loadSQLiteThresholdProbeConfig(t)
	if !config.Enabled {
		t.Skip("set enabled=true in sqliteThresholdProbeConfigJSON to measure SQLite size and query thresholds")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(positiveOrDefault(config.TimeoutSeconds, 180))*time.Second)
	defer cancel()

	outputDir := resolveProbeOutputDir(t, config.OutputDir, "sqlite-threshold")
	if config.CleanOutputDir {
		if err := os.RemoveAll(outputDir); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	results := make([]sqliteThresholdResult, 0, len(config.TradingDays))
	for _, tradingDays := range config.TradingDays {
		if tradingDays <= 0 {
			continue
		}
		dbPath := filepath.Join(outputDir, fmt.Sprintf("marketdata-%dd.db", tradingDays))
		rows, buildElapsed := buildSyntheticMarketDataDB(ctx, t, dbPath, config.Symbols, tradingDays)
		fileBytes, err := fileSize(dbPath)
		if err != nil {
			t.Fatal(err)
		}

		openStats := measureSQLiteOpen(ctx, t, dbPath, config.QueryRuns)
		countStats := measureSQLiteCountQuery(ctx, t, dbPath, config.QueryRuns)
		rankStats := measureSQLiteRankQuery(ctx, t, dbPath, config.QueryRuns, syntheticBasDt(tradingDays-1), config.TopN)

		result := sqliteThresholdResult{
			TradingDays: tradingDays,
			Rows:        rows,
			FileBytes:   fileBytes,
			OpenP50:     openStats.P50,
			CountP50:    countStats.P50,
			RankP50:     rankStats.P50,
		}
		results = append(results, result)

		t.Logf("days=%d rows=%d db_bytes=%d build=%s open_p50=%s count_p50=%s rank_top_%d_p50=%s path=%s",
			tradingDays,
			rows,
			fileBytes,
			formatDuration(buildElapsed),
			formatDuration(result.OpenP50),
			formatDuration(result.CountP50),
			config.TopN,
			formatDuration(result.RankP50),
			dbPath,
		)
	}

	resultsPath := filepath.Join(outputDir, "results.md")
	if err := writeSQLiteThresholdResults(resultsPath, config, results); err != nil {
		t.Fatal(err)
	}
	t.Logf("sqlite threshold results written to %s", resultsPath)
}

func loadSQLiteThresholdProbeConfig(t *testing.T) sqliteThresholdProbeConfig {
	t.Helper()
	var config sqliteThresholdProbeConfig
	if err := json.Unmarshal([]byte(sqliteThresholdProbeConfigJSON), &config); err != nil {
		t.Fatalf("decode sqliteThresholdProbeConfigJSON: %v", err)
	}
	if config.Symbols <= 0 {
		config.Symbols = 1100
	}
	if len(config.TradingDays) == 0 {
		config.TradingDays = []int{7, 30, 120, 250, 750}
	}
	if config.QueryRuns <= 0 {
		config.QueryRuns = 20
	}
	if config.TopN <= 0 {
		config.TopN = 100
	}
	return config
}

func buildSyntheticMarketDataDB(ctx context.Context, t *testing.T, dbPath string, symbols, tradingDays int) (int, time.Duration) {
	t.Helper()
	startedAt := time.Now()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=DELETE"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=OFF"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE instrument (
		srtn_cd TEXT NOT NULL PRIMARY KEY,
		isin_cd TEXT,
		itms_nm TEXT,
		bss_idx_idx_nm TEXT
	) WITHOUT ROWID`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE daily_etf_price (
		bas_dt INTEGER NOT NULL,
		srtn_cd TEXT NOT NULL,
		clpr INTEGER,
		flt_rt_bp INTEGER,
		trqu INTEGER,
		tr_prc INTEGER,
		nav_scaled INTEGER,
		PRIMARY KEY (bas_dt, srtn_cd)
	) WITHOUT ROWID`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX idx_daily_etf_price_return ON daily_etf_price (bas_dt, flt_rt_bp DESC)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "CREATE INDEX idx_daily_etf_price_volume ON daily_etf_price (bas_dt, trqu DESC)"); err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	instrumentStatement, err := tx.PrepareContext(ctx, "INSERT INTO instrument (srtn_cd, isin_cd, itms_nm, bss_idx_idx_nm) VALUES (?, ?, ?, ?)")
	if err != nil {
		t.Fatal(err)
	}
	for symbolIndex := range symbols {
		code := syntheticSymbol(symbolIndex)
		if _, err := instrumentStatement.ExecContext(ctx, code, "KR7"+code+"0000", "ETF "+code, "Synthetic Index"); err != nil {
			t.Fatal(err)
		}
	}
	if err := instrumentStatement.Close(); err != nil {
		t.Fatal(err)
	}

	priceStatement, err := tx.PrepareContext(ctx, `INSERT INTO daily_etf_price (
		bas_dt, srtn_cd, clpr, flt_rt_bp, trqu, tr_prc, nav_scaled
	) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for dayIndex := range tradingDays {
		basDt := syntheticBasDt(dayIndex)
		for symbolIndex := range symbols {
			closePrice := 5000 + symbolIndex%300*25 + dayIndex%90
			returnBP := (symbolIndex*37+dayIndex*11)%1800 - 900
			volume := int64(10_000 + (symbolIndex*997+dayIndex*101)%5_000_000)
			amount := volume * int64(closePrice)
			if _, err := priceStatement.ExecContext(ctx, basDt, syntheticSymbol(symbolIndex), closePrice, returnBP, volume, amount, closePrice*10_000); err != nil {
				t.Fatal(err)
			}
			rows++
		}
	}
	if err := priceStatement.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		t.Fatal(err)
	}
	return rows, time.Since(startedAt)
}

func measureSQLiteOpen(ctx context.Context, t *testing.T, dbPath string, runs int) coldStartStats {
	t.Helper()
	samples := make([]time.Duration, 0, runs)
	for range runs {
		startedAt := time.Now()
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.PingContext(ctx); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, time.Since(startedAt))
	}
	return summarizeColdStartSamples(samples)
}

func measureSQLiteCountQuery(ctx context.Context, t *testing.T, dbPath string, runs int) coldStartStats {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	samples := make([]time.Duration, 0, runs)
	for range runs {
		var count int64
		startedAt := time.Now()
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM daily_etf_price").Scan(&count); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, time.Since(startedAt))
	}
	return summarizeColdStartSamples(samples)
}

func measureSQLiteRankQuery(ctx context.Context, t *testing.T, dbPath string, runs, basDt, topN int) coldStartStats {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	samples := make([]time.Duration, 0, runs)
	for range runs {
		startedAt := time.Now()
		rows, err := db.QueryContext(ctx, `SELECT srtn_cd, flt_rt_bp
			FROM daily_etf_price
			WHERE bas_dt = ?
			ORDER BY flt_rt_bp DESC, srtn_cd ASC
			LIMIT ?`, basDt, topN)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var code string
			var value int64
			if err := rows.Scan(&code, &value); err != nil {
				rows.Close()
				t.Fatal(err)
			}
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		samples = append(samples, time.Since(startedAt))
	}
	return summarizeColdStartSamples(samples)
}

func writeSQLiteThresholdResults(path string, config sqliteThresholdProbeConfig, results []sqliteThresholdResult) error {
	var builder strings.Builder
	builder.WriteString("# SQLite Threshold Probe\n\n")
	builder.WriteString(fmt.Sprintf("- symbols: %d\n", config.Symbols))
	builder.WriteString(fmt.Sprintf("- top_n: %d\n", config.TopN))
	builder.WriteString(fmt.Sprintf("- query_runs: %d\n\n", config.QueryRuns))
	builder.WriteString("| trading_days | rows | db_bytes | db_mib | open_p50 | count_p50 | rank_p50 |\n")
	builder.WriteString("| ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, result := range results {
		builder.WriteString(fmt.Sprintf(
			"| %d | %d | %d | %.2f | %s | %s | %s |\n",
			result.TradingDays,
			result.Rows,
			result.FileBytes,
			float64(result.FileBytes)/(1024*1024),
			formatDuration(result.OpenP50),
			formatDuration(result.CountP50),
			formatDuration(result.RankP50),
		))
	}
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}

func syntheticBasDt(dayIndex int) int {
	return 20260000 + dayIndex + 1
}

func syntheticSymbol(symbolIndex int) string {
	return fmt.Sprintf("%06d", symbolIndex+1)
}
