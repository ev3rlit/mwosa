package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL     = "https://apis.data.go.kr/1160100/service/GetSecuritiesProductInfoService"
	defaultOutputDir   = "tmp/testing/datago-daily-json-collector/raw"
	providerID         = "datago"
	providerGroup      = "securitiesProductPrice"
	snapshotSchema     = 1
	defaultProductList = "etf,etn"
)

var productEndpoints = map[string]productEndpoint{
	"etf": {
		Product:   "etf",
		Operation: "getETFPriceInfo",
		Path:      "/getETFPriceInfo",
	},
	"etn": {
		Product:   "etn",
		Operation: "getETNPriceInfo",
		Path:      "/getETNPriceInfo",
	},
}

type collectorConfig struct {
	ServiceKey           string
	BaseURL              string
	OutputDir            string
	Products             []string
	StartDate            time.Time
	EndDate              time.Time
	Direction            string
	Compression          string
	NumRows              int
	Workers              int
	Retries              int
	RetryDelay           time.Duration
	RetryMaxDelay        time.Duration
	RequestTimeout       time.Duration
	DelayBetweenRequests time.Duration
	Overwrite            bool
	Pretty               bool
	Location             *time.Location
}

type productEndpoint struct {
	Product   string `json:"product"`
	Operation string `json:"operation"`
	Path      string `json:"path"`
}

type dailySnapshot struct {
	SchemaVersion int               `json:"schemaVersion"`
	Provider      string            `json:"provider"`
	Group         string            `json:"group"`
	BasDt         string            `json:"basDt"`
	FetchedAt     string            `json:"fetchedAt"`
	SourceBaseURL string            `json:"sourceBaseUrl"`
	Products      []productSnapshot `json:"products"`
}

type productSnapshot struct {
	Product    string            `json:"product"`
	Operation  string            `json:"operation"`
	Endpoint   string            `json:"endpoint"`
	ResultCode string            `json:"resultCode,omitempty"`
	ResultMsg  string            `json:"resultMsg,omitempty"`
	TotalCount int               `json:"totalCount"`
	PageCount  int               `json:"pageCount"`
	RowCount   int               `json:"rowCount"`
	Pages      []pageSnapshot    `json:"pages"`
	Items      []map[string]any  `json:"items"`
	Query      map[string]string `json:"query"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type pageSnapshot struct {
	PageNo     int `json:"pageNo"`
	NumOfRows  int `json:"numOfRows"`
	TotalCount int `json:"totalCount"`
	RowCount   int `json:"rowCount"`
}

type manifestEntry struct {
	Status     string                   `json:"status"`
	BasDt      string                   `json:"basDt"`
	Path       string                   `json:"path,omitempty"`
	FetchedAt  string                   `json:"fetchedAt"`
	Bytes      int64                    `json:"bytes,omitempty"`
	Products   []manifestProductSummary `json:"products,omitempty"`
	Error      string                   `json:"error,omitempty"`
	Skipped    bool                     `json:"skipped,omitempty"`
	Overwrote  bool                     `json:"overwrote,omitempty"`
	Compressed string                   `json:"compression,omitempty"`
}

type manifestProductSummary struct {
	Product    string `json:"product"`
	Operation  string `json:"operation"`
	TotalCount int    `json:"totalCount"`
	RowCount   int    `json:"rowCount"`
	PageCount  int    `json:"pageCount"`
}

type dailyCollectionJob struct {
	Index int
	Date  time.Time
}

type dailyCollectionResult struct {
	Index   int
	Total   int
	Worker  int
	BasDt   string
	Entry   manifestEntry
	Message string
	Err     error
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

func main() {
	cfg, err := parseConfig(os.Args[1:], time.Now())
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseConfig(args []string, now time.Time) (collectorConfig, error) {
	fs := flag.NewFlagSet("datago-daily-json-collector", flag.ContinueOnError)

	var (
		serviceKey     string
		baseURL        string
		outputDir      string
		products       string
		startDate      string
		endDate        string
		direction      string
		compression    string
		compress       bool
		timezone       string
		numRows        int
		workers        int
		retries        int
		retryDelay     time.Duration
		retryMaxDelay  time.Duration
		requestTimeout time.Duration
		requestDelay   time.Duration
		overwrite      bool
		pretty         bool
	)

	fs.StringVar(&serviceKey, "service-key", os.Getenv("DATAGO_SERVICE_KEY"), "data.go.kr service key, or DATAGO_SERVICE_KEY")
	fs.StringVar(&baseURL, "base-url", defaultBaseURL, "Datago securities product base URL")
	fs.StringVar(&outputDir, "output-dir", defaultOutputDir, "directory for date-partitioned JSON files")
	fs.StringVar(&products, "products", defaultProductList, "comma-separated products: etf,etn")
	fs.StringVar(&startDate, "start-date", "", "inclusive start date, YYYY-MM-DD or YYYYMMDD; defaults to one year before end-date")
	fs.StringVar(&endDate, "end-date", "", "inclusive end date, YYYY-MM-DD or YYYYMMDD; defaults to today")
	fs.StringVar(&direction, "direction", "desc", "collection order: desc or asc")
	fs.BoolVar(&compress, "compress", false, "gzip-compress output date files")
	fs.StringVar(&compression, "compression", "none", "output compression: none or gzip")
	fs.StringVar(&timezone, "timezone", "Asia/Seoul", "timezone used for default dates")
	fs.IntVar(&numRows, "num-rows", 1000, "Datago numOfRows per page")
	fs.IntVar(&workers, "workers", 1, "number of date workers")
	fs.IntVar(&retries, "retries", 3, "retries per Datago page request")
	fs.DurationVar(&retryDelay, "retry-delay", time.Second, "initial retry delay")
	fs.DurationVar(&retryMaxDelay, "retry-max-delay", 10*time.Second, "maximum retry delay")
	fs.DurationVar(&requestTimeout, "request-timeout", 30*time.Second, "HTTP client timeout per request")
	fs.DurationVar(&requestDelay, "request-delay", 200*time.Millisecond, "delay between API requests")
	fs.BoolVar(&overwrite, "overwrite", true, "overwrite existing date files; set false to skip")
	fs.BoolVar(&pretty, "pretty", false, "write indented JSON before compression")

	if err := fs.Parse(args); err != nil {
		return collectorConfig{}, err
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return collectorConfig{}, fmt.Errorf("load timezone %q: %w", timezone, err)
	}
	now = now.In(location)

	end, err := parseOptionalDate(endDate, now, location, "end-date")
	if err != nil {
		return collectorConfig{}, err
	}
	startFallback := end.AddDate(-1, 0, 0)
	start, err := parseOptionalDate(startDate, startFallback, location, "start-date")
	if err != nil {
		return collectorConfig{}, err
	}

	parsedProducts, err := parseProducts(products)
	if err != nil {
		return collectorConfig{}, err
	}
	if compress {
		compression = "gzip"
	}

	cfg := collectorConfig{
		ServiceKey:           strings.TrimSpace(serviceKey),
		BaseURL:              strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		OutputDir:            strings.TrimSpace(outputDir),
		Products:             parsedProducts,
		StartDate:            start,
		EndDate:              end,
		Direction:            strings.ToLower(strings.TrimSpace(direction)),
		Compression:          strings.ToLower(strings.TrimSpace(compression)),
		NumRows:              numRows,
		Workers:              workers,
		Retries:              retries,
		RetryDelay:           retryDelay,
		RetryMaxDelay:        retryMaxDelay,
		RequestTimeout:       requestTimeout,
		DelayBetweenRequests: requestDelay,
		Overwrite:            overwrite,
		Pretty:               pretty,
		Location:             location,
	}
	if err := validateConfig(cfg); err != nil {
		return collectorConfig{}, err
	}
	return cfg, nil
}

func validateConfig(cfg collectorConfig) error {
	if cfg.ServiceKey == "" {
		return errors.New("service key is required: pass --service-key or DATAGO_SERVICE_KEY")
	}
	if cfg.BaseURL == "" {
		return errors.New("base-url is required")
	}
	if cfg.OutputDir == "" {
		return errors.New("output-dir is required")
	}
	if cfg.NumRows <= 0 {
		return fmt.Errorf("num-rows must be positive: %d", cfg.NumRows)
	}
	if cfg.Workers <= 0 {
		return fmt.Errorf("workers must be positive: %d", cfg.Workers)
	}
	if cfg.Retries < 0 {
		return fmt.Errorf("retries must be zero or positive: %d", cfg.Retries)
	}
	if cfg.RetryDelay < 0 {
		return fmt.Errorf("retry-delay must be zero or positive: %s", cfg.RetryDelay)
	}
	if cfg.RetryMaxDelay < 0 {
		return fmt.Errorf("retry-max-delay must be zero or positive: %s", cfg.RetryMaxDelay)
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("request-timeout must be positive: %s", cfg.RequestTimeout)
	}
	if cfg.StartDate.After(cfg.EndDate) {
		return fmt.Errorf("start-date must be on or before end-date: start=%s end=%s", formatDate(cfg.StartDate), formatDate(cfg.EndDate))
	}
	if cfg.Direction != "asc" && cfg.Direction != "desc" {
		return fmt.Errorf("direction must be asc or desc: %q", cfg.Direction)
	}
	if cfg.Compression != "gzip" && cfg.Compression != "none" {
		return fmt.Errorf("compression must be gzip or none: %q", cfg.Compression)
	}
	return nil
}

func run(ctx context.Context, cfg collectorConfig) error {
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir %s: %w", cfg.OutputDir, err)
	}

	client := &http.Client{Timeout: cfg.RequestTimeout}
	dates := enumerateDates(cfg.StartDate, cfg.EndDate, cfg.Direction)
	workerCount := cfg.Workers
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(dates) {
		workerCount = len(dates)
	}
	if workerCount <= 0 {
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan dailyCollectionJob)
	results := make(chan dailyCollectionResult)
	var wg sync.WaitGroup
	for workerID := 1; workerID <= workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobs {
				result := processDaily(workerCtx, client, cfg, job, len(dates), workerID)
				select {
				case results <- result:
				case <-workerCtx.Done():
					return
				}
				if result.Err != nil {
					cancel()
					return
				}
			}
		}(workerID)
	}

	go func() {
		defer close(jobs)
		for index, date := range dates {
			select {
			case jobs <- dailyCollectionJob{Index: index + 1, Date: date}:
			case <-workerCtx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for result := range results {
		if err := appendManifest(cfg.OutputDir, result.Entry); err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
		if result.Message != "" {
			fmt.Println(result.Message)
		}
		if result.Err != nil && firstErr == nil {
			firstErr = result.Err
			cancel()
		}
	}
	return firstErr
}

func processDaily(ctx context.Context, client *http.Client, cfg collectorConfig, job dailyCollectionJob, total, workerID int) dailyCollectionResult {
	basDt := formatBasDt(job.Date)
	outputPath := dailyFilePath(cfg.OutputDir, job.Date, cfg.Compression)
	fetchedAt := time.Now().In(cfg.Location).Format(time.RFC3339)

	if !cfg.Overwrite && fileExists(outputPath) {
		entry := manifestEntry{
			Status:     "skipped",
			BasDt:      basDt,
			Path:       outputPath,
			FetchedAt:  fetchedAt,
			Skipped:    true,
			Compressed: cfg.Compression,
		}
		return dailyCollectionResult{
			Index:   job.Index,
			Total:   total,
			Worker:  workerID,
			BasDt:   basDt,
			Entry:   entry,
			Message: fmt.Sprintf("[%d/%d worker=%d] skipped basDt=%s path=%s", job.Index, total, workerID, basDt, outputPath),
		}
	}

	overwrote := fileExists(outputPath)
	snapshot, err := collectDailySnapshot(ctx, client, cfg, job.Date)
	if err != nil {
		entry := manifestEntry{
			Status:     "error",
			BasDt:      basDt,
			Path:       outputPath,
			FetchedAt:  fetchedAt,
			Error:      err.Error(),
			Compressed: cfg.Compression,
		}
		return dailyCollectionResult{
			Index:   job.Index,
			Total:   total,
			Worker:  workerID,
			BasDt:   basDt,
			Entry:   entry,
			Message: fmt.Sprintf("[%d/%d worker=%d] error basDt=%s error=%s", job.Index, total, workerID, basDt, err),
			Err:     fmt.Errorf("collect basDt=%s: %w", basDt, err),
		}
	}

	writtenBytes, err := writeDailySnapshot(outputPath, snapshot, cfg)
	if err != nil {
		entry := manifestEntry{
			Status:     "error",
			BasDt:      basDt,
			Path:       outputPath,
			FetchedAt:  fetchedAt,
			Error:      err.Error(),
			Compressed: cfg.Compression,
		}
		return dailyCollectionResult{
			Index:   job.Index,
			Total:   total,
			Worker:  workerID,
			BasDt:   basDt,
			Entry:   entry,
			Message: fmt.Sprintf("[%d/%d worker=%d] error basDt=%s error=%s", job.Index, total, workerID, basDt, err),
			Err:     fmt.Errorf("write basDt=%s: %w", basDt, err),
		}
	}

	entry := manifestEntry{
		Status:     "ok",
		BasDt:      basDt,
		Path:       outputPath,
		FetchedAt:  snapshot.FetchedAt,
		Bytes:      writtenBytes,
		Products:   summarizeProducts(snapshot.Products),
		Overwrote:  overwrote,
		Compressed: cfg.Compression,
	}
	return dailyCollectionResult{
		Index:   job.Index,
		Total:   total,
		Worker:  workerID,
		BasDt:   basDt,
		Entry:   entry,
		Message: fmt.Sprintf("[%d/%d worker=%d] wrote basDt=%s rows=%s bytes=%d path=%s", job.Index, total, workerID, basDt, formatProductCounts(snapshot.Products), writtenBytes, outputPath),
	}
}

func collectDailySnapshot(ctx context.Context, client *http.Client, cfg collectorConfig, date time.Time) (dailySnapshot, error) {
	basDt := formatBasDt(date)
	snapshot := dailySnapshot{
		SchemaVersion: snapshotSchema,
		Provider:      providerID,
		Group:         providerGroup,
		BasDt:         basDt,
		FetchedAt:     time.Now().In(cfg.Location).Format(time.RFC3339),
		SourceBaseURL: cfg.BaseURL,
		Products:      make([]productSnapshot, 0, len(cfg.Products)),
	}

	for index, product := range cfg.Products {
		endpoint := productEndpoints[product]
		productResult, err := fetchProduct(ctx, client, cfg, endpoint, basDt)
		if err != nil {
			return dailySnapshot{}, err
		}
		snapshot.Products = append(snapshot.Products, productResult)

		if cfg.DelayBetweenRequests > 0 && index < len(cfg.Products)-1 {
			if err := sleepWithContext(ctx, cfg.DelayBetweenRequests); err != nil {
				return dailySnapshot{}, err
			}
		}
	}
	return snapshot, nil
}

func fetchProduct(ctx context.Context, client *http.Client, cfg collectorConfig, endpoint productEndpoint, basDt string) (productSnapshot, error) {
	result := productSnapshot{
		Product:   endpoint.Product,
		Operation: endpoint.Operation,
		Endpoint:  endpoint.Path,
		Pages:     []pageSnapshot{},
		Items:     []map[string]any{},
		Query: map[string]string{
			"basDt":      basDt,
			"numOfRows":  strconv.Itoa(cfg.NumRows),
			"resultType": "json",
		},
		Metadata: map[string]string{
			"provider": providerID,
			"group":    providerGroup,
		},
	}

	for pageNo := 1; ; pageNo++ {
		page, err := fetchProductPage(ctx, client, cfg, endpoint, basDt, pageNo)
		if err != nil {
			return productSnapshot{}, err
		}
		result.ResultCode = page.Header.ResultCode
		result.ResultMsg = page.Header.ResultMsg
		result.TotalCount = page.Body.TotalCount
		result.Items = append(result.Items, page.Body.Items.Item...)
		result.Pages = append(result.Pages, pageSnapshot{
			PageNo:     page.Body.PageNo,
			NumOfRows:  page.Body.NumOfRows,
			TotalCount: page.Body.TotalCount,
			RowCount:   len(page.Body.Items.Item),
		})
		result.PageCount = len(result.Pages)
		result.RowCount = len(result.Items)

		if result.RowCount >= result.TotalCount || len(page.Body.Items.Item) == 0 {
			return result, nil
		}
		if cfg.DelayBetweenRequests > 0 {
			if err := sleepWithContext(ctx, cfg.DelayBetweenRequests); err != nil {
				return productSnapshot{}, err
			}
		}
	}
}

func fetchProductPage(ctx context.Context, client *http.Client, cfg collectorConfig, endpoint productEndpoint, basDt string, pageNo int) (datagoEnvelope, error) {
	requestURL, err := buildRequestURL(cfg, endpoint, basDt, pageNo)
	if err != nil {
		return datagoEnvelope{}, err
	}
	attempts := cfg.Retries + 1
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		envelope, retryable, err := fetchProductPageOnce(ctx, client, requestURL, cfg, endpoint, pageNo)
		if err == nil {
			return envelope, nil
		}
		lastErr = err
		if !retryable || attempt == attempts {
			break
		}
		delay := retryDelay(cfg, attempt)
		fmt.Fprintf(os.Stderr, "retrying %s basDt=%s pageNo=%d attempt=%d/%d delay=%s error=%s\n", endpoint.Operation, basDt, pageNo, attempt+1, attempts, delay, err)
		if delay > 0 {
			if err := sleepWithContext(ctx, delay); err != nil {
				return datagoEnvelope{}, err
			}
		}
	}
	return datagoEnvelope{}, fmt.Errorf("%s basDt=%s pageNo=%d failed after %d attempt(s): %w", endpoint.Operation, basDt, pageNo, attempts, lastErr)
}

func fetchProductPageOnce(ctx context.Context, client *http.Client, requestURL string, cfg collectorConfig, endpoint productEndpoint, pageNo int) (datagoEnvelope, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return datagoEnvelope{}, false, err
	}

	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return datagoEnvelope{}, false, ctx.Err()
		}
		return datagoEnvelope{}, true, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		if ctx.Err() != nil {
			return datagoEnvelope{}, false, ctx.Err()
		}
		return datagoEnvelope{}, true, err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return datagoEnvelope{}, isRetryableHTTPStatus(response.StatusCode), fmt.Errorf("%s failed: status=%s body=%s", endpoint.Operation, response.Status, trimForError(body))
	}

	envelope, err := decodeDatagoEnvelope(body)
	if err != nil {
		return datagoEnvelope{}, false, err
	}
	if envelope.Header.ResultCode != "" && envelope.Header.ResultCode != "00" {
		return datagoEnvelope{}, isRetryableResult(envelope.Header), fmt.Errorf("%s returned resultCode=%s resultMsg=%s", endpoint.Operation, envelope.Header.ResultCode, envelope.Header.ResultMsg)
	}
	if envelope.Body.PageNo == 0 {
		envelope.Body.PageNo = pageNo
	}
	if envelope.Body.NumOfRows == 0 {
		envelope.Body.NumOfRows = cfg.NumRows
	}
	return envelope, false, nil
}

func buildRequestURL(cfg collectorConfig, endpoint productEndpoint, basDt string, pageNo int) (string, error) {
	parsedURL, err := url.Parse(cfg.BaseURL + endpoint.Path)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	query.Set("serviceKey", normalizeDatagoServiceKey(cfg.ServiceKey))
	query.Set("resultType", "json")
	query.Set("basDt", basDt)
	query.Set("pageNo", strconv.Itoa(pageNo))
	query.Set("numOfRows", strconv.Itoa(cfg.NumRows))
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func retryDelay(cfg collectorConfig, completedRetries int) time.Duration {
	delay := cfg.RetryDelay
	for i := 1; i < completedRetries; i++ {
		delay *= 2
	}
	if cfg.RetryMaxDelay > 0 && delay > cfg.RetryMaxDelay {
		return cfg.RetryMaxDelay
	}
	return delay
}

func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isRetryableResult(header datagoHeader) bool {
	if header.ResultCode == "22" {
		return true
	}
	message := strings.ToLower(header.ResultMsg)
	return strings.Contains(message, "limit") || strings.Contains(message, "timeout") || strings.Contains(message, "temporar")
}

func (items *datagoItems) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`""`)) {
		return nil
	}

	var wrapper struct {
		Item json.RawMessage `json:"item"`
	}
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return err
	}
	itemData := bytes.TrimSpace(wrapper.Item)
	if len(itemData) == 0 || bytes.Equal(itemData, []byte("null")) || bytes.Equal(itemData, []byte(`""`)) {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(itemData))
	decoder.UseNumber()
	if bytes.HasPrefix(itemData, []byte("[")) {
		return decoder.Decode(&items.Item)
	}

	var item map[string]any
	if err := decoder.Decode(&item); err != nil {
		return err
	}
	items.Item = []map[string]any{item}
	return nil
}

func decodeDatagoEnvelope(body []byte) (datagoEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()

	var root struct {
		Response *datagoEnvelope `json:"response"`
		datagoEnvelope
	}
	if err := decoder.Decode(&root); err != nil {
		return datagoEnvelope{}, fmt.Errorf("decode datago JSON response: %w; body=%s", err, trimForError(body))
	}
	if root.Response != nil {
		return *root.Response, nil
	}
	return root.datagoEnvelope, nil
}

func writeDailySnapshot(path string, snapshot dailySnapshot, cfg collectorConfig) (int64, error) {
	var (
		payload []byte
		err     error
	)
	if cfg.Pretty {
		payload, err = json.MarshalIndent(snapshot, "", "  ")
	} else {
		payload, err = json.Marshal(snapshot)
	}
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	tempPath := fmt.Sprintf("%s.tmp-%d", path, os.Getpid())
	if err := writePayload(tempPath, payload, cfg.Compression); err != nil {
		_ = os.Remove(tempPath)
		return 0, err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func writePayload(path string, payload []byte, compression string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	var writeErr error
	switch compression {
	case "gzip":
		writer := gzip.NewWriter(file)
		if _, writeErr = writer.Write(payload); writeErr == nil {
			writeErr = writer.Close()
		} else {
			_ = writer.Close()
		}
	case "none":
		_, writeErr = file.Write(payload)
	default:
		writeErr = fmt.Errorf("unsupported compression %q", compression)
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func appendManifest(outputDir string, entry manifestEntry) error {
	manifestPath := filepath.Join(outputDir, "manifest.jsonl")
	file, err := os.OpenFile(manifestPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open manifest %s: %w", manifestPath, err)
	}
	defer file.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(line); err != nil {
		return err
	}
	if _, err := file.WriteString("\n"); err != nil {
		return err
	}
	return nil
}

func summarizeProducts(products []productSnapshot) []manifestProductSummary {
	summaries := make([]manifestProductSummary, 0, len(products))
	for _, product := range products {
		summaries = append(summaries, manifestProductSummary{
			Product:    product.Product,
			Operation:  product.Operation,
			TotalCount: product.TotalCount,
			RowCount:   product.RowCount,
			PageCount:  product.PageCount,
		})
	}
	return summaries
}

func formatProductCounts(products []productSnapshot) string {
	parts := make([]string, 0, len(products))
	for _, product := range products {
		parts = append(parts, fmt.Sprintf("%s=%d", product.Product, product.RowCount))
	}
	return strings.Join(parts, ",")
}

func parseOptionalDate(value string, fallback time.Time, location *time.Location, name string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return startOfDay(fallback.In(location), location), nil
	}
	for _, layout := range []string{"2006-01-02", "20060102"} {
		parsed, err := time.ParseInLocation(layout, value, location)
		if err == nil {
			return startOfDay(parsed, location), nil
		}
	}
	return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD or YYYYMMDD: %q", name, value)
}

func parseProducts(value string) ([]string, error) {
	seen := make(map[string]struct{})
	var products []string
	for _, part := range strings.Split(value, ",") {
		product := strings.ToLower(strings.TrimSpace(part))
		if product == "" {
			continue
		}
		if _, ok := productEndpoints[product]; !ok {
			return nil, fmt.Errorf("unsupported product %q; supported products are etf,etn", product)
		}
		if _, ok := seen[product]; ok {
			continue
		}
		seen[product] = struct{}{}
		products = append(products, product)
	}
	if len(products) == 0 {
		return nil, errors.New("products must include at least one of etf,etn")
	}
	return products, nil
}

func enumerateDates(start, end time.Time, direction string) []time.Time {
	var dates []time.Time
	if direction == "desc" {
		for date := end; !date.Before(start); date = date.AddDate(0, 0, -1) {
			dates = append(dates, date)
		}
		return dates
	}
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		dates = append(dates, date)
	}
	return dates
}

func dailyFilePath(outputDir string, date time.Time, compression string) string {
	basDt := formatBasDt(date)
	extension := ".json"
	if compression == "gzip" {
		extension = ".json.gz"
	}
	return filepath.Join(outputDir, date.Format("2006"), date.Format("01"), basDt+extension)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func startOfDay(value time.Time, location *time.Location) time.Time {
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func formatDate(date time.Time) string {
	return date.Format("2006-01-02")
}

func formatBasDt(date time.Time) string {
	return date.Format("20060102")
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func normalizeDatagoServiceKey(serviceKey string) string {
	if strings.Contains(serviceKey, "%") {
		if decoded, err := url.QueryUnescape(serviceKey); err == nil {
			return decoded
		}
	}
	return serviceKey
}

func trimForError(body []byte) string {
	const max = 500
	body = bytes.TrimSpace(body)
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}
