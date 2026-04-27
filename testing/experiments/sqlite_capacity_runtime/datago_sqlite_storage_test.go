package sqlite_capacity_runtime

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/samber/oops"
	_ "modernc.org/sqlite"
)

const probeConfigJSON = `
{
  "datago": {
    "etf_price_info_url": "https://apis.data.go.kr/1160100/service/GetSecuritiesProductInfoService/getETFPriceInfo",
    "service_key": "",
    "bas_dt": "",
    "begin_bas_dt": "20260417",
    "end_bas_dt": "20260424",
    "sample_rows": 10,
    "num_rows": 1000
  },
  "sqlite": {
    "enabled": true,
    "output_dir": "tmp/testing/sqlite-capacity-runtime/datago-sqlite",
    "clean_output_dir": true,
    "database_file": "mwosa-etf-daily.db",
    "instrument_table": "instrument",
    "daily_price_table": "daily_etf_price",
    "ranking_table": "leader_snapshot",
    "ranking_limit": 100,
    "test_timeout_seconds": 180
  }
}
`

type probeConfig struct {
	Datago datagoProbeConfig `json:"datago"`
	SQLite sqliteProbeConfig `json:"sqlite"`
}

type datagoProbeConfig struct {
	ETFPriceInfoURL string `json:"etf_price_info_url"`
	ServiceKey      string `json:"service_key"`
	BasDt           string `json:"bas_dt"`
	BeginBasDt      string `json:"begin_bas_dt"`
	EndBasDt        string `json:"end_bas_dt"`
	SampleRows      int    `json:"sample_rows"`
	NumRows         int    `json:"num_rows"`
}

type sqliteProbeConfig struct {
	Enabled            bool   `json:"enabled"`
	OutputDir          string `json:"output_dir"`
	CleanOutputDir     bool   `json:"clean_output_dir"`
	DatabaseFile       string `json:"database_file"`
	InstrumentTable    string `json:"instrument_table"`
	DailyPriceTable    string `json:"daily_price_table"`
	RankingTable       string `json:"ranking_table"`
	RankingLimit       int    `json:"ranking_limit"`
	TestTimeoutSeconds int    `json:"test_timeout_seconds"`
}

type datagoEnvelope struct {
	Header datagoHeader `json:"header"`
	Body   datagoBody   `json:"body"`
}

type datagoHeader struct {
	ResultCode string `json:"resultCode"`
	ResultMsg  string `json:"resultMsg"`
}

type datagoBody struct {
	NumOfRows  int         `json:"numOfRows"`
	PageNo     int         `json:"pageNo"`
	TotalCount int         `json:"totalCount"`
	Items      datagoItems `json:"items"`
}

type datagoItems struct {
	Item []map[string]any `json:"item"`
}

func (items *datagoItems) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}

	var wrapper struct {
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	if len(wrapper.Item) == 0 || bytes.Equal(bytes.TrimSpace(wrapper.Item), []byte("null")) {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(wrapper.Item))
	decoder.UseNumber()

	if bytes.HasPrefix(bytes.TrimSpace(wrapper.Item), []byte("[")) {
		return decoder.Decode(&items.Item)
	}

	var item map[string]any
	if err := decoder.Decode(&item); err != nil {
		return err
	}
	items.Item = []map[string]any{item}
	return nil
}

func TestDatagoETFPriceInfoFetchShape(t *testing.T) {
	config := loadProbeConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, totalCount, err := fetchDatagoETFPriceInfoPage(ctx, http.DefaultClient, config.Datago, datagoFetchOptions{
		DateRange: config.Datago.DateRange(),
		PageNo:    1,
		NumOfRows: positiveOrDefault(config.Datago.SampleRows, 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one ETF row, got 0 rows; totalCount=%d dateRange=%s", totalCount, config.Datago.DateRange())
	}

	first := rows[0]
	for _, key := range []string{"basDt", "srtnCd", "itmsNm", "clpr", "fltRt", "trqu", "trPrc"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("expected first ETF row to include %q; keys=%v", key, mapKeys(first))
		}
	}
	t.Logf("fetched ETF sample rows=%d totalCount=%d dateRange=%s first=%v/%v/%v", len(rows), totalCount, config.Datago.DateRange(), first["basDt"], first["srtnCd"], first["itmsNm"])
}

func TestDatagoETFPriceInfoSQLiteStorageSize(t *testing.T) {
	config := loadProbeConfig(t)
	if !config.SQLite.Enabled {
		t.Skip("set sqlite.enabled=true in probeConfigJSON to store fetched ETF rows in SQLite")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(positiveOrDefault(config.SQLite.TestTimeoutSeconds, 180))*time.Second)
	defer cancel()

	dateRange := config.Datago.DateRange()
	rows, err := fetchAllDatagoETFPriceInfo(ctx, http.DefaultClient, config.Datago, dateRange, positiveOrDefault(config.Datago.NumRows, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one ETF row for dateRange=%s", dateRange)
	}
	basDts := uniqueBasDts(rows)

	outputDir := resolveProbeOutputDir(t, config.SQLite.OutputDir, "datago-sqlite")
	if config.SQLite.CleanOutputDir {
		if err := os.RemoveAll(outputDir); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ndjsonPath := filepath.Join(outputDir, "datago-etf-price-info.ndjson")
	ndjsonBytes, err := writeNDJSON(ndjsonPath, rows)
	if err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(outputDir, config.SQLite.DatabaseFile)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := setupSQLiteProbeSchema(ctx, db, config.SQLite); err != nil {
		t.Fatal(err)
	}
	if err := insertSQLiteInstruments(ctx, db, config.SQLite.InstrumentTable, rows); err != nil {
		t.Fatal(err)
	}
	if err := insertSQLiteETFPriceRows(ctx, db, config.SQLite.DailyPriceTable, rows); err != nil {
		t.Fatal(err)
	}
	if err := refreshSQLiteRankingSnapshots(ctx, db, config.SQLite, basDts); err != nil {
		t.Fatal(err)
	}
	rankingRows, err := countSQLiteRankingSnapshots(ctx, db, config.SQLite.RankingTable, basDts)
	if err != nil {
		t.Fatal(err)
	}
	if rankingRows == 0 {
		t.Fatalf("expected ranking_snapshot rows for dateRange=%s", dateRange)
	}
	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	sqliteBytes, err := fileSize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("datago ETF snapshot dateRange=%s basDt_count=%d basDts=%s rows=%d", dateRange, len(basDts), strings.Join(basDts, ","), len(rows))
	t.Logf("sqlite leader_snapshot rows=%d metrics=return_1d,volume,traded_amount ranking_limit=%d", rankingRows, config.SQLite.RankingLimit)
	t.Logf("raw NDJSON bytes=%d bytes_per_row=%.1f path=%s", ndjsonBytes, float64(ndjsonBytes)/float64(len(rows)), ndjsonPath)
	t.Logf("sqlite bytes=%d bytes_per_row=%.1f ratio_to_ndjson=%.2f path=%s", sqliteBytes, float64(sqliteBytes)/float64(len(rows)), float64(sqliteBytes)/float64(ndjsonBytes), dbPath)
}

type datagoFetchOptions struct {
	DateRange datagoDateRange
	PageNo    int
	NumOfRows int
}

type datagoDateRange struct {
	BasDt      string
	BeginBasDt string
	EndBasDt   string
}

func (config datagoProbeConfig) DateRange() datagoDateRange {
	return datagoDateRange{
		BasDt:      strings.TrimSpace(config.BasDt),
		BeginBasDt: strings.TrimSpace(config.BeginBasDt),
		EndBasDt:   strings.TrimSpace(config.EndBasDt),
	}
}

func (dateRange datagoDateRange) String() string {
	if dateRange.BeginBasDt != "" || dateRange.EndBasDt != "" {
		return fmt.Sprintf("beginBasDt=%q endBasDt=%q", dateRange.BeginBasDt, dateRange.EndBasDt)
	}
	return fmt.Sprintf("basDt=%q", dateRange.BasDt)
}

func fetchAllDatagoETFPriceInfo(ctx context.Context, client *http.Client, config datagoProbeConfig, dateRange datagoDateRange, numOfRows int) ([]map[string]any, error) {
	if numOfRows <= 0 {
		return nil, oops.In("sqlite_capacity_runtime").With("num_of_rows", numOfRows).Errorf("numOfRows must be positive: %d", numOfRows)
	}

	var allRows []map[string]any
	for pageNo := 1; ; pageNo++ {
		rows, totalCount, err := fetchDatagoETFPriceInfoPage(ctx, client, config, datagoFetchOptions{
			DateRange: dateRange,
			PageNo:    pageNo,
			NumOfRows: numOfRows,
		})
		if err != nil {
			return nil, err
		}
		allRows = append(allRows, rows...)
		if len(allRows) >= totalCount || len(rows) == 0 {
			return allRows, nil
		}
	}
}

func fetchDatagoETFPriceInfoPage(ctx context.Context, client *http.Client, config datagoProbeConfig, opts datagoFetchOptions) ([]map[string]any, int, error) {
	endpoint, err := url.Parse(config.ETFPriceInfoURL)
	if err != nil {
		return nil, 0, err
	}
	query := endpoint.Query()
	query.Set("serviceKey", normalizeDatagoServiceKey(config.ServiceKey))
	query.Set("resultType", "json")
	query.Set("pageNo", strconv.Itoa(opts.PageNo))
	query.Set("numOfRows", strconv.Itoa(opts.NumOfRows))
	if opts.DateRange.BeginBasDt != "" || opts.DateRange.EndBasDt != "" {
		if opts.DateRange.BeginBasDt != "" {
			query.Set("beginBasDt", opts.DateRange.BeginBasDt)
		}
		if opts.DateRange.EndBasDt != "" {
			query.Set("endBasDt", opts.DateRange.EndBasDt)
		}
	} else if opts.DateRange.BasDt != "" {
		query.Set("basDt", opts.DateRange.BasDt)
	}
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, 0, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, 0, oops.In("sqlite_capacity_runtime").With("status", response.Status, "body", trimForError(body)).New("datago getETFPriceInfo failed")
	}

	envelope, err := decodeDatagoEnvelope(body)
	if err != nil {
		return nil, 0, err
	}
	if envelope.Header.ResultCode != "" && envelope.Header.ResultCode != "00" {
		return nil, 0, oops.In("sqlite_capacity_runtime").With("result_code", envelope.Header.ResultCode, "result_msg", envelope.Header.ResultMsg).New("datago getETFPriceInfo returned non-ok result code")
	}
	return envelope.Body.Items.Item, envelope.Body.TotalCount, nil
}

func decodeDatagoEnvelope(body []byte) (datagoEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var root struct {
		Response *datagoEnvelope `json:"response"`
		datagoEnvelope
	}
	if err := decoder.Decode(&root); err != nil {
		return datagoEnvelope{}, oops.In("sqlite_capacity_runtime").With("body", trimForError(body)).Wrapf(err, "decode datago JSON response")
	}
	if root.Response != nil {
		return *root.Response, nil
	}
	return root.datagoEnvelope, nil
}

func loadProbeConfig(t *testing.T) probeConfig {
	t.Helper()
	var config probeConfig
	if err := json.Unmarshal([]byte(probeConfigJSON), &config); err != nil {
		t.Fatalf("decode probeConfigJSON: %v", err)
	}
	if config.Datago.ETFPriceInfoURL == "" {
		t.Fatal("probeConfigJSON datago.etf_price_info_url is required")
	}
	if config.Datago.ServiceKey == "" {
		t.Skip("set probeConfigJSON datago.service_key to run data.go.kr integration probes")
	}
	if config.Datago.BasDt == "" && config.Datago.BeginBasDt == "" && config.Datago.EndBasDt == "" {
		t.Fatal("probeConfigJSON requires datago.bas_dt or datago.begin_bas_dt/datago.end_bas_dt")
	}
	if config.SQLite.InstrumentTable == "" {
		config.SQLite.InstrumentTable = "instrument"
	}
	if config.SQLite.DailyPriceTable == "" {
		config.SQLite.DailyPriceTable = "daily_etf_price"
	}
	if config.SQLite.RankingTable == "" {
		config.SQLite.RankingTable = "leader_snapshot"
	}
	if config.SQLite.RankingLimit <= 0 {
		config.SQLite.RankingLimit = 100
	}
	if config.SQLite.DatabaseFile == "" {
		config.SQLite.DatabaseFile = "mwosa-etf-daily.db"
	}
	return config
}

func setupSQLiteProbeSchema(ctx context.Context, db *sql.DB, config sqliteProbeConfig) error {
	statements := []string{
		"PRAGMA journal_mode=DELETE",
		"PRAGMA synchronous=FULL",
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			srtn_cd TEXT NOT NULL,
			isin_cd TEXT,
			itms_nm TEXT,
			bss_idx_idx_nm TEXT,
			PRIMARY KEY (srtn_cd)
		)`, config.InstrumentTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			bas_dt TEXT NOT NULL,
			srtn_cd TEXT NOT NULL,
			clpr REAL,
			vs REAL,
			flt_rt REAL,
			mkp REAL,
			hipr REAL,
			lopr REAL,
			trqu INTEGER,
			tr_prc INTEGER,
			mrkt_tot_amt INTEGER,
			n_ppt_tot_amt INTEGER,
			st_lstg_cnt INTEGER,
			nav REAL,
			bss_idx_clpr REAL,
			PRIMARY KEY (bas_dt, srtn_cd)
		)`, config.DailyPriceTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_bas_dt_flt_rt ON %s (bas_dt, flt_rt DESC)", config.DailyPriceTable, config.DailyPriceTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_bas_dt_trqu ON %s (bas_dt, trqu DESC)", config.DailyPriceTable, config.DailyPriceTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_bas_dt_tr_prc ON %s (bas_dt, tr_prc DESC)", config.DailyPriceTable, config.DailyPriceTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			bas_dt TEXT NOT NULL,
			metric TEXT NOT NULL,
			rank INTEGER NOT NULL,
			srtn_cd TEXT NOT NULL,
			itms_nm TEXT,
			value REAL,
			PRIMARY KEY (bas_dt, metric, rank, srtn_cd)
		)`, config.RankingTable),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_lookup ON %s (bas_dt, metric, rank)", config.RankingTable, config.RankingTable),
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func insertSQLiteInstruments(ctx context.Context, db *sql.DB, table string, rows []map[string]any) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statement, err := tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (
		srtn_cd, isin_cd, itms_nm, bss_idx_idx_nm
	) VALUES (?, ?, ?, ?)
	ON CONFLICT(srtn_cd) DO UPDATE SET
		isin_cd = excluded.isin_cd,
		itms_nm = excluded.itms_nm,
		bss_idx_idx_nm = excluded.bss_idx_idx_nm`, table))
	if err != nil {
		return err
	}
	defer statement.Close()

	for _, row := range rows {
		if _, err := statement.ExecContext(
			ctx,
			textValue(row["srtnCd"]),
			textValue(row["isinCd"]),
			textValue(row["itmsNm"]),
			textValue(row["bssIdxIdxNm"]),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertSQLiteETFPriceRows(ctx context.Context, db *sql.DB, table string, rows []map[string]any) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statement, err := tx.PrepareContext(ctx, fmt.Sprintf(`INSERT INTO %s (
		bas_dt, srtn_cd, clpr, vs, flt_rt, mkp, hipr, lopr,
		trqu, tr_prc, mrkt_tot_amt, n_ppt_tot_amt, st_lstg_cnt, nav,
		bss_idx_clpr
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(bas_dt, srtn_cd) DO UPDATE SET
		clpr = excluded.clpr,
		vs = excluded.vs,
		flt_rt = excluded.flt_rt,
		mkp = excluded.mkp,
		hipr = excluded.hipr,
		lopr = excluded.lopr,
		trqu = excluded.trqu,
		tr_prc = excluded.tr_prc,
		mrkt_tot_amt = excluded.mrkt_tot_amt,
		n_ppt_tot_amt = excluded.n_ppt_tot_amt,
		st_lstg_cnt = excluded.st_lstg_cnt,
		nav = excluded.nav,
		bss_idx_clpr = excluded.bss_idx_clpr`, table))
	if err != nil {
		return err
	}
	defer statement.Close()

	for _, row := range rows {
		if _, err := statement.ExecContext(
			ctx,
			textValue(row["basDt"]),
			textValue(row["srtnCd"]),
			realValue(row["clpr"]),
			realValue(row["vs"]),
			realValue(row["fltRt"]),
			realValue(row["mkp"]),
			realValue(row["hipr"]),
			realValue(row["lopr"]),
			intValue(row["trqu"]),
			intValue(row["trPrc"]),
			intValue(row["mrktTotAmt"]),
			intValue(row["nPptTotAmt"]),
			intValue(row["stLstgCnt"]),
			realValue(row["nav"]),
			realValue(row["bssIdxClpr"]),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func refreshSQLiteRankingSnapshots(ctx context.Context, db *sql.DB, config sqliteProbeConfig, basDts []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, basDt := range basDts {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE bas_dt = ?", config.RankingTable), basDt); err != nil {
			return err
		}
	}

	rankingQueries := []struct {
		metric string
		column string
	}{
		{metric: "return_1d", column: "flt_rt"},
		{metric: "volume", column: "trqu"},
		{metric: "traded_amount", column: "tr_prc"},
	}

	for _, basDt := range basDts {
		for _, ranking := range rankingQueries {
			query := fmt.Sprintf(`INSERT INTO %s (bas_dt, metric, rank, srtn_cd, itms_nm, value)
				SELECT bas_dt, ?, rank, srtn_cd, itms_nm, value
				FROM (
					SELECT
						p.bas_dt,
						p.srtn_cd,
						i.itms_nm,
						p.%s AS value,
						RANK() OVER (PARTITION BY p.bas_dt ORDER BY p.%s DESC, p.srtn_cd ASC) AS rank
					FROM %s p
					LEFT JOIN %s i ON i.srtn_cd = p.srtn_cd
					WHERE p.bas_dt = ? AND p.%s IS NOT NULL
				)
				WHERE rank <= ?`, config.RankingTable, ranking.column, ranking.column, config.DailyPriceTable, config.InstrumentTable, ranking.column)
			if _, err := tx.ExecContext(ctx, query, ranking.metric, basDt, config.RankingLimit); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func countSQLiteRankingSnapshots(ctx context.Context, db *sql.DB, table string, basDts []string) (int64, error) {
	var count int64
	for _, basDt := range basDts {
		var basDtCount int64
		if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT count(*) FROM %s WHERE bas_dt = ?", table), basDt).Scan(&basDtCount); err != nil {
			return 0, err
		}
		count += basDtCount
	}
	return count, nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func textValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		if typed == "" {
			return nil
		}
		return typed
	case json.Number:
		return typed.String()
	default:
		text := fmt.Sprint(typed)
		if text == "" || text == "<nil>" {
			return nil
		}
		return text
	}
}

func realValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return nil
		}
		return parsed
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		if typed == "" {
			return nil
		}
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return nil
		}
		return parsed
	default:
		return nil
	}
}

func intValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed
		}
		floatParsed, err := typed.Float64()
		if err != nil {
			return nil
		}
		return int64(floatParsed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case string:
		if typed == "" {
			return nil
		}
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err == nil {
			return parsed
		}
		floatParsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return nil
		}
		return int64(floatParsed)
	default:
		return nil
	}
}

func uniqueBasDts(rows []map[string]any) []string {
	seen := make(map[string]struct{})
	var basDts []string
	for _, row := range rows {
		basDt, ok := textValue(row["basDt"]).(string)
		if !ok || basDt == "" {
			continue
		}
		if _, exists := seen[basDt]; exists {
			continue
		}
		seen[basDt] = struct{}{}
		basDts = append(basDts, basDt)
	}
	return basDts
}

func normalizeDatagoServiceKey(serviceKey string) string {
	if strings.Contains(serviceKey, "%") {
		if decoded, err := url.QueryUnescape(serviceKey); err == nil {
			return decoded
		}
	}
	return serviceKey
}

func writeNDJSON(path string, rows []map[string]any) (int64, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	var written int64
	for _, row := range rows {
		line, err := json.Marshal(row)
		if err != nil {
			return 0, err
		}
		n, err := writer.Write(line)
		written += int64(n)
		if err != nil {
			return 0, err
		}
		n, err = writer.WriteString("\n")
		written += int64(n)
		if err != nil {
			return 0, err
		}
	}
	if err := writer.Flush(); err != nil {
		return 0, err
	}
	return written, nil
}

func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return total, err
}

func positiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func trimForError(body []byte) string {
	const max = 500
	body = bytes.TrimSpace(body)
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}
