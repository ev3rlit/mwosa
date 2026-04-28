package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCollectDailySnapshotFetchesETFAndETN(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("serviceKey") != "decoded key" {
			t.Fatalf("serviceKey was not normalized before query encoding: %q", r.URL.RawQuery)
		}
		if r.URL.Query().Get("resultType") != "json" {
			t.Fatalf("resultType=%q, want json", r.URL.Query().Get("resultType"))
		}
		if r.URL.Query().Get("basDt") != "20260428" {
			t.Fatalf("basDt=%q, want 20260428", r.URL.Query().Get("basDt"))
		}

		switch r.URL.Path {
		case "/getETFPriceInfo":
			writeDatagoResponse(t, w, datagoTestResponse{
				PageNo:     mustAtoi(r.URL.Query().Get("pageNo")),
				NumOfRows:  1,
				TotalCount: 2,
				ItemJSON: map[int]string{
					1: `{"basDt":"20260428","srtnCd":"069500","itmsNm":"KODEX 200"}`,
					2: `{"basDt":"20260428","srtnCd":"102110","itmsNm":"TIGER 200"}`,
				}[mustAtoi(r.URL.Query().Get("pageNo"))],
			})
		case "/getETNPriceInfo":
			writeDatagoResponse(t, w, datagoTestResponse{
				PageNo:     1,
				NumOfRows:  1,
				TotalCount: 1,
				ItemJSON:   `[{"basDt":"20260428","srtnCd":"570001","itmsNm":"TRUE ETN"}]`,
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	location := time.FixedZone("KST", 9*60*60)
	cfg := collectorConfig{
		ServiceKey:           "decoded%20key",
		BaseURL:              server.URL,
		OutputDir:            t.TempDir(),
		Products:             []string{"etf", "etn"},
		StartDate:            time.Date(2026, 4, 28, 0, 0, 0, 0, location),
		EndDate:              time.Date(2026, 4, 28, 0, 0, 0, 0, location),
		Direction:            "desc",
		Compression:          "none",
		NumRows:              1,
		Workers:              1,
		RequestTimeout:       5 * time.Second,
		DelayBetweenRequests: 0,
		Location:             location,
	}

	if err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}

	snapshotPath := dailyFilePath(cfg.OutputDir, cfg.EndDate, cfg.Compression)
	snapshot := readSnapshot(t, snapshotPath)
	if snapshot.Provider != providerID {
		t.Fatalf("provider=%q, want %q", snapshot.Provider, providerID)
	}
	if snapshot.BasDt != "20260428" {
		t.Fatalf("basDt=%q, want 20260428", snapshot.BasDt)
	}
	if len(snapshot.Products) != 2 {
		t.Fatalf("products=%d, want 2", len(snapshot.Products))
	}
	if snapshot.Products[0].Product != "etf" || snapshot.Products[0].RowCount != 2 || snapshot.Products[0].PageCount != 2 {
		t.Fatalf("unexpected ETF snapshot: %+v", snapshot.Products[0])
	}
	if snapshot.Products[1].Product != "etn" || snapshot.Products[1].RowCount != 1 || snapshot.Products[1].PageCount != 1 {
		t.Fatalf("unexpected ETN snapshot: %+v", snapshot.Products[1])
	}

	manifestPath := filepath.Join(cfg.OutputDir, "manifest.jsonl")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifestBytes) == 0 {
		t.Fatal("manifest.jsonl is empty")
	}
}

func TestParseConfigDefaultsToOneYearDescendingUncompressedJSON(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]string{"--service-key", "test"}, time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := formatDate(cfg.EndDate), "2026-04-28"; got != want {
		t.Fatalf("end date=%s, want %s", got, want)
	}
	if got, want := formatDate(cfg.StartDate), "2025-04-28"; got != want {
		t.Fatalf("start date=%s, want %s", got, want)
	}
	if cfg.Direction != "desc" {
		t.Fatalf("direction=%q, want desc", cfg.Direction)
	}
	if cfg.Compression != "none" {
		t.Fatalf("compression=%q, want none", cfg.Compression)
	}
	if cfg.Workers != 1 {
		t.Fatalf("workers=%d, want 1", cfg.Workers)
	}
	if cfg.Retries != 3 {
		t.Fatalf("retries=%d, want 3", cfg.Retries)
	}
	if !cfg.Overwrite {
		t.Fatal("overwrite=false, want true by default")
	}
}

func TestParseConfigCompressFlagUsesGzip(t *testing.T) {
	t.Parallel()

	cfg, err := parseConfig([]string{"--service-key", "test", "--compress"}, time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Compression != "gzip" {
		t.Fatalf("compression=%q, want gzip", cfg.Compression)
	}
}

func TestWriteDailySnapshotSupportsGzip(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("KST", 9*60*60)
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, location)
	cfg := collectorConfig{
		OutputDir:   t.TempDir(),
		Compression: "gzip",
		Location:    location,
	}
	snapshot := dailySnapshot{
		SchemaVersion: snapshotSchema,
		Provider:      providerID,
		Group:         providerGroup,
		BasDt:         "20260428",
		FetchedAt:     "2026-04-28T00:00:00+09:00",
		SourceBaseURL: defaultBaseURL,
	}

	path := dailyFilePath(cfg.OutputDir, date, cfg.Compression)
	if _, err := writeDailySnapshot(path, snapshot, cfg); err != nil {
		t.Fatal(err)
	}
	readBack := readGzipSnapshot(t, path)
	if readBack.BasDt != snapshot.BasDt {
		t.Fatalf("basDt=%q, want %q", readBack.BasDt, snapshot.BasDt)
	}
}

func TestRunRetriesTransientPageFailures(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "temporary outage", http.StatusServiceUnavailable)
			return
		}
		writeDatagoResponse(t, w, datagoTestResponse{
			PageNo:     1,
			NumOfRows:  1,
			TotalCount: 1,
			ItemJSON:   `{"basDt":"20260428","srtnCd":"069500","itmsNm":"KODEX 200"}`,
		})
	}))
	t.Cleanup(server.Close)

	location := time.FixedZone("KST", 9*60*60)
	cfg := collectorConfig{
		ServiceKey:           "test",
		BaseURL:              server.URL,
		OutputDir:            t.TempDir(),
		Products:             []string{"etf"},
		StartDate:            time.Date(2026, 4, 28, 0, 0, 0, 0, location),
		EndDate:              time.Date(2026, 4, 28, 0, 0, 0, 0, location),
		Direction:            "desc",
		Compression:          "none",
		NumRows:              1,
		Workers:              1,
		Retries:              1,
		RetryDelay:           0,
		RetryMaxDelay:        0,
		RequestTimeout:       5 * time.Second,
		DelayBetweenRequests: 0,
		Location:             location,
	}

	if err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests=%d, want 2", got)
	}
}

func TestRunUsesDateWorkers(t *testing.T) {
	t.Parallel()

	var current atomic.Int32
	var maxConcurrent atomic.Int32
	var started atomic.Int32
	var releaseOnce sync.Once
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inFlight := current.Add(1)
		defer current.Add(-1)
		for {
			previous := maxConcurrent.Load()
			if inFlight <= previous || maxConcurrent.CompareAndSwap(previous, inFlight) {
				break
			}
		}
		if started.Add(1) == 2 {
			releaseOnce.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
			http.Error(w, "date workers did not overlap", http.StatusInternalServerError)
			return
		}
		writeDatagoResponse(t, w, datagoTestResponse{
			PageNo:     1,
			NumOfRows:  1000,
			TotalCount: 1,
			ItemJSON:   fmt.Sprintf(`{"basDt":%q,"srtnCd":"069500","itmsNm":"KODEX 200"}`, r.URL.Query().Get("basDt")),
		})
	}))
	t.Cleanup(server.Close)

	location := time.FixedZone("KST", 9*60*60)
	cfg := collectorConfig{
		ServiceKey:           "test",
		BaseURL:              server.URL,
		OutputDir:            t.TempDir(),
		Products:             []string{"etf"},
		StartDate:            time.Date(2026, 4, 27, 0, 0, 0, 0, location),
		EndDate:              time.Date(2026, 4, 28, 0, 0, 0, 0, location),
		Direction:            "desc",
		Compression:          "none",
		NumRows:              1000,
		Workers:              2,
		Retries:              0,
		RequestTimeout:       5 * time.Second,
		DelayBetweenRequests: 0,
		Location:             location,
	}

	if err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if got := maxConcurrent.Load(); got < 2 {
		t.Fatalf("max concurrent requests=%d, want at least 2", got)
	}
}

func TestRunSkipsExistingFileWhenOverwriteDisabled(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "should not fetch existing files", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	location := time.FixedZone("KST", 9*60*60)
	date := time.Date(2026, 4, 28, 0, 0, 0, 0, location)
	outputDir := t.TempDir()
	outputPath := dailyFilePath(outputDir, date, "none")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte(`{"existing":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := collectorConfig{
		ServiceKey:           "test",
		BaseURL:              server.URL,
		OutputDir:            outputDir,
		Products:             []string{"etf"},
		StartDate:            date,
		EndDate:              date,
		Direction:            "desc",
		Compression:          "none",
		NumRows:              1000,
		Workers:              1,
		Retries:              0,
		RequestTimeout:       5 * time.Second,
		DelayBetweenRequests: 0,
		Overwrite:            false,
		Location:             location,
	}

	if err := run(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("requests=%d, want 0", got)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"existing":true}` {
		t.Fatalf("existing file was modified: %s", data)
	}
}

type datagoTestResponse struct {
	PageNo     int
	NumOfRows  int
	TotalCount int
	ItemJSON   string
}

func writeDatagoResponse(t *testing.T, w http.ResponseWriter, response datagoTestResponse) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"response": {
			"header": {"resultCode":"00","resultMsg":"NORMAL SERVICE."},
			"body": {
				"numOfRows": %d,
				"pageNo": %d,
				"totalCount": %d,
				"items": {"item": %s}
			}
		}
	}`, response.NumOfRows, response.PageNo, response.TotalCount, response.ItemJSON)
}

func readSnapshot(t *testing.T, path string) dailySnapshot {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot dailySnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func readGzipSnapshot(t *testing.T, path string) dailySnapshot {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot dailySnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func mustAtoi(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		panic(err)
	}
	return parsed
}
