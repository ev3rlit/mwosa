package stockprice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samber/oops"
)

type Client struct {
	serviceKey       string
	baseURL          string
	httpClient       *http.Client
	retryMaxAttempts int
	retryInitialWait time.Duration
	retryMaxWait     time.Duration
}

func New(config Config) (*Client, error) {
	config = config.withDefaults()
	errb := oops.In("datago_stock_price_client").With(
		"provider", ProviderDataGo,
		"group", GroupStockPrice,
	)

	if strings.TrimSpace(config.ServiceKey) == "" {
		return nil, errb.New("datago stock price client config: serviceKey is required")
	}
	if _, err := url.ParseRequestURI(config.BaseURL); err != nil {
		return nil, errb.With("base_url", config.BaseURL).Wrapf(err, "datago stock price client config: invalid baseURL")
	}
	return &Client{
		serviceKey:       config.ServiceKey,
		baseURL:          strings.TrimRight(config.BaseURL, "/"),
		httpClient:       config.HTTPClient,
		retryMaxAttempts: config.RetryMaxAttempts,
		retryInitialWait: config.RetryInitialWait,
		retryMaxWait:     config.RetryMaxWait,
	}, nil
}

type StockPriceInfoResult struct {
	Items      []StockPriceInfo
	NumOfRows  int
	PageNo     int
	TotalCount int
}

func (c *Client) GetStockPriceInfo(ctx context.Context, query StockPriceInfoQuery) (StockPriceInfoResult, error) {
	result, err := fetchPriceInfoPage(c, ctx, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return StockPriceInfoResult{}, err
	}
	return StockPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetAllStockPriceInfo(ctx context.Context, query StockPriceInfoQuery) (StockPriceInfoResult, error) {
	query = query.forAllPages()
	result, err := fetchAllPriceInfoPages(c, ctx, query.values(), query.numOfRows(), query.workers())
	if err != nil {
		return StockPriceInfoResult{}, err
	}
	return StockPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

type priceInfoResult struct {
	Items      []StockPriceInfo
	NumOfRows  int
	PageNo     int
	TotalCount int
}

func fetchPriceInfoPage(c *Client, ctx context.Context, params url.Values, pageNo int, numOfRows int) (priceInfoResult, error) {
	errb := oops.In("datago_stock_price_client").With(
		"provider", ProviderDataGo,
		"group", GroupStockPrice,
		"operation", OperationGetStockPriceInfo,
	)

	response, err := fetchPage(c, ctx, params, pageNo, numOfRows)
	if err != nil {
		return priceInfoResult{}, errb.With("page", pageNo).Wrapf(err, "fetch datago stock price info page")
	}
	return priceInfoResult{
		Items:      response.Body.Items,
		NumOfRows:  response.Body.NumOfRows,
		PageNo:     response.Body.PageNo,
		TotalCount: response.Body.TotalCount,
	}, nil
}

func fetchAllPriceInfoPages(c *Client, ctx context.Context, params url.Values, numOfRows int, workers int) (priceInfoResult, error) {
	errb := oops.In("datago_stock_price_client").With(
		"provider", ProviderDataGo,
		"group", GroupStockPrice,
		"operation", OperationGetStockPriceInfo,
		"workers", workers,
	)

	first, err := fetchPriceInfoPage(c, ctx, params, 1, numOfRows)
	if err != nil {
		return priceInfoResult{}, errb.With("page", 1).Wrapf(err, "fetch datago stock price info first page")
	}

	items := append([]StockPriceInfo(nil), first.Items...)
	effectiveNumOfRows := first.NumOfRows
	if effectiveNumOfRows <= 0 {
		effectiveNumOfRows = numOfRows
	}
	pageCount := pageCount(first.TotalCount, effectiveNumOfRows)
	if pageCount <= 1 {
		return priceInfoResult{
			Items:      items,
			NumOfRows:  effectiveNumOfRows,
			PageNo:     1,
			TotalCount: first.TotalCount,
		}, nil
	}
	if workers <= 1 {
		for pageNo := 2; pageNo <= pageCount; pageNo++ {
			next, err := fetchPriceInfoPage(c, ctx, params, pageNo, numOfRows)
			if err != nil {
				return priceInfoResult{}, errb.With("page", pageNo).Wrapf(err, "fetch datago stock price info page")
			}
			items = append(items, next.Items...)
		}
		return priceInfoResult{
			Items:      items,
			NumOfRows:  effectiveNumOfRows,
			PageNo:     1,
			TotalCount: first.TotalCount,
		}, nil
	}

	remaining, err := fetchRemainingPriceInfoPages(c, ctx, params, numOfRows, pageCount, workers)
	if err != nil {
		return priceInfoResult{}, err
	}
	for pageNo := 2; pageNo <= pageCount; pageNo++ {
		items = append(items, remaining[pageNo]...)
	}

	return priceInfoResult{
		Items:      items,
		NumOfRows:  effectiveNumOfRows,
		PageNo:     1,
		TotalCount: first.TotalCount,
	}, nil
}

type priceInfoPageResult struct {
	pageNo int
	items  []StockPriceInfo
	err    error
}

func fetchRemainingPriceInfoPages(c *Client, ctx context.Context, params url.Values, numOfRows int, pageCount int, workers int) (map[int][]StockPriceInfo, error) {
	errb := oops.In("datago_stock_price_client").With(
		"provider", ProviderDataGo,
		"group", GroupStockPrice,
		"operation", OperationGetStockPriceInfo,
		"workers", workers,
	)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	results := make(chan priceInfoPageResult)
	var wg sync.WaitGroup
	for workerID := 0; workerID < workers; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pageNo := range jobs {
				next, err := fetchPriceInfoPage(c, workerCtx, params, pageNo, numOfRows)
				result := priceInfoPageResult{pageNo: pageNo, err: err}
				if err == nil {
					result.items = next.Items
				}
				select {
				case results <- result:
				case <-workerCtx.Done():
					return
				}
				if err != nil {
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for pageNo := 2; pageNo <= pageCount; pageNo++ {
			select {
			case jobs <- pageNo:
			case <-workerCtx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	pages := make(map[int][]StockPriceInfo, pageCount-1)
	var firstErr error
	for result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = errb.With("page", result.pageNo).Wrapf(result.err, "fetch datago stock price info page")
				cancel()
			}
			continue
		}
		pages[result.pageNo] = result.items
	}
	if firstErr != nil {
		return nil, firstErr
	}
	for pageNo := 2; pageNo <= pageCount; pageNo++ {
		if _, ok := pages[pageNo]; !ok {
			return nil, errb.With("page", pageNo).New("datago stock price info page result missing")
		}
	}
	return pages, nil
}

func fetchPage(c *Client, ctx context.Context, params url.Values, pageNo int, numOfRows int) (apiResponse, error) {
	errb := oops.In("datago_stock_price_client").With(
		"provider", ProviderDataGo,
		"group", GroupStockPrice,
		"operation", OperationGetStockPriceInfo,
		"page", pageNo,
	)

	maxAttempts := c.retryMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		decoded, retryable, err := fetchPageOnce(c, ctx, params, pageNo, numOfRows)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts {
			return apiResponse{}, err
		}
		if err := sleepBeforeRetry(ctx, retryDelay(c, attempt)); err != nil {
			return apiResponse{}, errb.With("attempt", attempt).Wrap(oops.Join(lastErr, err))
		}
	}
	return apiResponse{}, errb.Wrap(lastErr)
}

func fetchPageOnce(c *Client, ctx context.Context, params url.Values, pageNo int, numOfRows int) (apiResponse, bool, error) {
	errb := oops.In("datago_stock_price_client").With(
		"provider", ProviderDataGo,
		"group", GroupStockPrice,
		"operation", OperationGetStockPriceInfo,
		"page", pageNo,
	)

	endpoint := fmt.Sprintf("%s/%s", c.baseURL, OperationGetStockPriceInfo)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return apiResponse{}, false, errb.Wrapf(err, "datago stock price request build failed")
	}

	values := cloneValues(params)
	values.Set("serviceKey", c.serviceKey)
	values.Set("resultType", "json")
	values.Set("numOfRows", strconv.Itoa(numOfRows))
	values.Set("pageNo", strconv.Itoa(pageNo))
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return apiResponse{}, false, errb.Wrapf(err, "datago stock price request build failed")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse{}, shouldRetryRequestError(ctx), errb.Wrapf(err, "datago stock price remote request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, true, errb.With("status", resp.StatusCode).Wrapf(err, "datago stock price remote response read failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyText := trimForError(body)
		return apiResponse{}, shouldRetryStatus(resp.StatusCode), errb.With("status", resp.StatusCode, "body", bodyText).Errorf("datago remote error provider=%s group=%s operation=%s page=%d status=%d body=%s", ProviderDataGo, GroupStockPrice, OperationGetStockPriceInfo, pageNo, resp.StatusCode, bodyText)
	}

	decoded, err := decodeAPIResponse(body)
	if err != nil {
		return apiResponse{}, false, errb.Wrapf(err, "datago stock price response decode failed")
	}
	if decoded.Header.ResultCode != "" && decoded.Header.ResultCode != "00" {
		return apiResponse{}, false, errb.With("result_code", decoded.Header.ResultCode, "result_msg", decoded.Header.ResultMsg).Errorf("datago remote error provider=%s group=%s operation=%s page=%d result_code=%s result_msg=%s", ProviderDataGo, GroupStockPrice, OperationGetStockPriceInfo, pageNo, decoded.Header.ResultCode, decoded.Header.ResultMsg)
	}
	return decoded, false, nil
}

func shouldRetryRequestError(ctx context.Context) bool {
	return ctx.Err() == nil
}

func shouldRetryStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func retryDelay(c *Client, attempt int) time.Duration {
	delay := c.retryInitialWait
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= c.retryMaxWait {
			return c.retryMaxWait
		}
	}
	if delay > c.retryMaxWait {
		return c.retryMaxWait
	}
	return delay
}

func sleepBeforeRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func trimForError(body []byte) string {
	bodyText := strings.TrimSpace(string(body))
	const limit = 1000
	if len(bodyText) <= limit {
		return bodyText
	}
	return bodyText[:limit] + "...(truncated)"
}

func pageCount(totalCount int, numOfRows int) int {
	if totalCount <= 0 || numOfRows <= 0 {
		return 0
	}
	return (totalCount + numOfRows - 1) / numOfRows
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

type apiResponse struct {
	Header apiHeader
	Body   apiBody
}

type apiHeader struct {
	ResultCode string `json:"resultCode"`
	ResultMsg  string `json:"resultMsg"`
}

type apiBody struct {
	NumOfRows  int
	PageNo     int
	TotalCount int
	Items      []StockPriceInfo
}

func decodeAPIResponse(body []byte) (apiResponse, error) {
	errb := oops.In("datago_stock_price_client")
	type responseEnvelope struct {
		Header apiHeader `json:"header"`
		Body   struct {
			NumOfRows  int `json:"numOfRows"`
			PageNo     int `json:"pageNo"`
			TotalCount int `json:"totalCount"`
			Items      struct {
				Item json.RawMessage `json:"item"`
			} `json:"items"`
		} `json:"body"`
	}
	var raw struct {
		Response *responseEnvelope `json:"response"`
		responseEnvelope
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return apiResponse{}, errb.Wrapf(err, "decode datago stock price JSON envelope")
	}
	response := raw.Response
	if response == nil {
		if raw.Header.ResultCode == "" && raw.Header.ResultMsg == "" && raw.Body.PageNo == 0 && raw.Body.NumOfRows == 0 && raw.Body.TotalCount == 0 {
			return apiResponse{}, errb.New("decode datago stock price JSON envelope: response is required")
		}
		response = &raw.responseEnvelope
	}

	items, err := decodeItems(response.Body.Items.Item)
	if err != nil {
		return apiResponse{}, errb.Wrapf(err, "decode datago stock price items")
	}
	return apiResponse{
		Header: response.Header,
		Body: apiBody{
			NumOfRows:  response.Body.NumOfRows,
			PageNo:     response.Body.PageNo,
			TotalCount: response.Body.TotalCount,
			Items:      items,
		},
	}, nil
}

func decodeItems(raw json.RawMessage) ([]StockPriceInfo, error) {
	errb := oops.In("datago_stock_price_client")
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	switch trimmed[0] {
	case '[':
		var items []StockPriceInfo
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, errb.Wrapf(err, "decode datago stock price item array")
		}
		return items, nil
	case '{':
		var item StockPriceInfo
		if err := json.Unmarshal(trimmed, &item); err != nil {
			return nil, errb.Wrapf(err, "decode datago stock price item object")
		}
		return []StockPriceInfo{item}, nil
	default:
		return nil, errb.With("item", string(trimmed)).Errorf("unsupported item shape: %s", string(trimmed))
	}
}
