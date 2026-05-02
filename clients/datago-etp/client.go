package etp

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
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
	)

	if strings.TrimSpace(config.ServiceKey) == "" {
		return nil, errb.New("datago client config: serviceKey is required")
	}
	if _, err := url.ParseRequestURI(config.BaseURL); err != nil {
		return nil, errb.With("base_url", config.BaseURL).Wrapf(err, "datago client config: invalid baseURL")
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

type ETFPriceInfoResult struct {
	Items      []ETFPriceInfo
	NumOfRows  int
	PageNo     int
	TotalCount int
}

type ETNPriceInfoResult struct {
	Items      []ETNPriceInfo
	NumOfRows  int
	PageNo     int
	TotalCount int
}

type ELWPriceInfoResult struct {
	Items      []ELWPriceInfo
	NumOfRows  int
	PageNo     int
	TotalCount int
}

type PriceInfoMetadata struct {
	TotalCount     int
	PageSize       int
	PageCount      int
	ProbePageNo    int
	ProbeNumOfRows int
}

func (c *Client) GetETFPriceInfo(ctx context.Context, query ETFPriceInfoQuery) (ETFPriceInfoResult, error) {
	result, err := fetchPriceInfoPage[ETFPriceInfo](c, ctx, OperationGetETFPriceInfo, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return ETFPriceInfoResult{}, err
	}
	return ETFPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetETFPriceInfoMetadata(ctx context.Context, query ETFPriceInfoQuery) (PriceInfoMetadata, error) {
	probeQuery, pageSize := query.forMetadataProbe()
	return fetchPriceInfoMetadata[ETFPriceInfo](c, ctx, OperationGetETFPriceInfo, probeQuery.values(), pageSize)
}

func (c *Client) GetAllETFPriceInfo(ctx context.Context, query ETFPriceInfoQuery) (ETFPriceInfoResult, error) {
	query.SecuritiesProductPriceQuery = query.forAllPages()
	result, err := fetchAllPriceInfoPages[ETFPriceInfo](c, ctx, OperationGetETFPriceInfo, query.values(), query.numOfRows(), query.workers())
	if err != nil {
		return ETFPriceInfoResult{}, err
	}
	return ETFPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetETNPriceInfo(ctx context.Context, query ETNPriceInfoQuery) (ETNPriceInfoResult, error) {
	result, err := fetchPriceInfoPage[ETNPriceInfo](c, ctx, OperationGetETNPriceInfo, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return ETNPriceInfoResult{}, err
	}
	return ETNPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetETNPriceInfoMetadata(ctx context.Context, query ETNPriceInfoQuery) (PriceInfoMetadata, error) {
	probeQuery, pageSize := query.forMetadataProbe()
	return fetchPriceInfoMetadata[ETNPriceInfo](c, ctx, OperationGetETNPriceInfo, probeQuery.values(), pageSize)
}

func (c *Client) GetAllETNPriceInfo(ctx context.Context, query ETNPriceInfoQuery) (ETNPriceInfoResult, error) {
	query.SecuritiesProductPriceQuery = query.forAllPages()
	result, err := fetchAllPriceInfoPages[ETNPriceInfo](c, ctx, OperationGetETNPriceInfo, query.values(), query.numOfRows(), query.workers())
	if err != nil {
		return ETNPriceInfoResult{}, err
	}
	return ETNPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetELWPriceInfo(ctx context.Context, query ELWPriceInfoQuery) (ELWPriceInfoResult, error) {
	result, err := fetchPriceInfoPage[ELWPriceInfo](c, ctx, OperationGetELWPriceInfo, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return ELWPriceInfoResult{}, err
	}
	return ELWPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetELWPriceInfoMetadata(ctx context.Context, query ELWPriceInfoQuery) (PriceInfoMetadata, error) {
	probeQuery, pageSize := query.forMetadataProbe()
	return fetchPriceInfoMetadata[ELWPriceInfo](c, ctx, OperationGetELWPriceInfo, probeQuery.values(), pageSize)
}

func (c *Client) GetAllELWPriceInfo(ctx context.Context, query ELWPriceInfoQuery) (ELWPriceInfoResult, error) {
	query.SecuritiesProductPriceQuery = query.forAllPages()
	result, err := fetchAllPriceInfoPages[ELWPriceInfo](c, ctx, OperationGetELWPriceInfo, query.values(), query.numOfRows(), query.workers())
	if err != nil {
		return ELWPriceInfoResult{}, err
	}
	return ELWPriceInfoResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

type priceInfoResult[T any] struct {
	Items      []T
	NumOfRows  int
	PageNo     int
	TotalCount int
}

func fetchPriceInfoPage[T any](c *Client, ctx context.Context, operation string, params url.Values, pageNo int, numOfRows int) (priceInfoResult[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
	)

	if strings.TrimSpace(operation) == "" {
		return priceInfoResult[T]{}, errb.New("datago price info operation is required")
	}

	response, err := fetchPage[T](c, ctx, operation, params, pageNo, numOfRows)
	if err != nil {
		return priceInfoResult[T]{}, errb.With("page", pageNo).Wrapf(err, "fetch datago price info page")
	}
	return priceInfoResult[T]{
		Items:      response.Body.Items,
		NumOfRows:  response.Body.NumOfRows,
		PageNo:     response.Body.PageNo,
		TotalCount: response.Body.TotalCount,
	}, nil
}

func fetchPriceInfoMetadata[T any](c *Client, ctx context.Context, operation string, params url.Values, pageSize int) (PriceInfoMetadata, error) {
	result, err := fetchPriceInfoPage[T](c, ctx, operation, params, 1, 1)
	if err != nil {
		return PriceInfoMetadata{}, err
	}
	return PriceInfoMetadata{
		TotalCount:     result.TotalCount,
		PageSize:       pageSize,
		PageCount:      pageCount(result.TotalCount, pageSize),
		ProbePageNo:    result.PageNo,
		ProbeNumOfRows: result.NumOfRows,
	}, nil
}

func fetchAllPriceInfoPages[T any](c *Client, ctx context.Context, operation string, params url.Values, numOfRows int, workers int) (priceInfoResult[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
		"workers", workers,
	)

	first, err := fetchPriceInfoPage[T](c, ctx, operation, params, 1, numOfRows)
	if err != nil {
		return priceInfoResult[T]{}, errb.With("page", 1).Wrapf(err, "fetch datago price info first page")
	}

	items := append([]T(nil), first.Items...)
	effectiveNumOfRows := first.NumOfRows
	if effectiveNumOfRows <= 0 {
		effectiveNumOfRows = numOfRows
	}
	pageCount := pageCount(first.TotalCount, effectiveNumOfRows)
	if pageCount <= 1 {
		return priceInfoResult[T]{
			Items:      items,
			NumOfRows:  effectiveNumOfRows,
			PageNo:     1,
			TotalCount: first.TotalCount,
		}, nil
	}
	if workers <= 1 {
		for pageNo := 2; pageNo <= pageCount; pageNo++ {
			next, err := fetchPriceInfoPage[T](c, ctx, operation, params, pageNo, numOfRows)
			if err != nil {
				return priceInfoResult[T]{}, errb.With("page", pageNo).Wrapf(err, "fetch datago price info page")
			}
			items = append(items, next.Items...)
		}
		return priceInfoResult[T]{
			Items:      items,
			NumOfRows:  effectiveNumOfRows,
			PageNo:     1,
			TotalCount: first.TotalCount,
		}, nil
	}

	remaining, err := fetchRemainingPriceInfoPages[T](c, ctx, operation, params, numOfRows, pageCount, workers)
	if err != nil {
		return priceInfoResult[T]{}, err
	}
	for pageNo := 2; pageNo <= pageCount; pageNo++ {
		items = append(items, remaining[pageNo]...)
	}

	return priceInfoResult[T]{
		Items:      items,
		NumOfRows:  effectiveNumOfRows,
		PageNo:     1,
		TotalCount: first.TotalCount,
	}, nil
}

type priceInfoPageResult[T any] struct {
	pageNo int
	items  []T
	err    error
}

func fetchRemainingPriceInfoPages[T any](c *Client, ctx context.Context, operation string, params url.Values, numOfRows int, pageCount int, workers int) (map[int][]T, error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
		"workers", workers,
	)
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	results := make(chan priceInfoPageResult[T])
	var wg sync.WaitGroup
	for workerID := 0; workerID < workers; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pageNo := range jobs {
				next, err := fetchPriceInfoPage[T](c, workerCtx, operation, params, pageNo, numOfRows)
				result := priceInfoPageResult[T]{pageNo: pageNo, err: err}
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

	pages := make(map[int][]T, pageCount-1)
	var firstErr error
	for result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = errb.With("page", result.pageNo).Wrapf(result.err, "fetch datago price info page")
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
			return nil, errb.With("page", pageNo).New("datago price info page result missing")
		}
	}
	return pages, nil
}

func fetchPage[T any](c *Client, ctx context.Context, operation string, params url.Values, pageNo int, numOfRows int) (apiResponse[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
		"page", pageNo,
	)

	maxAttempts := c.retryMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		decoded, retryable, err := fetchPageOnce[T](c, ctx, operation, params, pageNo, numOfRows)
		if err == nil {
			return decoded, nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts {
			return apiResponse[T]{}, err
		}
		if err := sleepBeforeRetry(ctx, retryDelay(c, attempt)); err != nil {
			return apiResponse[T]{}, errb.With("attempt", attempt).Wrap(oops.Join(lastErr, err))
		}
	}
	return apiResponse[T]{}, errb.Wrap(lastErr)
}

func fetchPageOnce[T any](c *Client, ctx context.Context, operation string, params url.Values, pageNo int, numOfRows int) (apiResponse[T], bool, error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
		"page", pageNo,
	)

	endpoint := fmt.Sprintf("%s/%s", c.baseURL, operation)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return apiResponse[T]{}, false, errb.Wrapf(err, "datago request build failed")
	}

	values := cloneValues(params)
	values.Set("serviceKey", c.serviceKey)
	// Datago defaults to XML when resultType is omitted; this client only supports JSON parsing.
	values.Set("resultType", "json")
	values.Set("numOfRows", strconv.Itoa(numOfRows))
	values.Set("pageNo", strconv.Itoa(pageNo))
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return apiResponse[T]{}, false, errb.Wrapf(err, "datago request build failed")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse[T]{}, shouldRetryRequestError(ctx), errb.Wrapf(err, "datago remote request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse[T]{}, true, errb.With("status", resp.StatusCode).Wrapf(err, "datago remote response read failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyText := trimForError(body)
		return apiResponse[T]{}, shouldRetryStatus(resp.StatusCode), errb.With("status", resp.StatusCode, "body", bodyText).Errorf("datago remote error provider=%s group=%s operation=%s page=%d status=%d body=%s", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, resp.StatusCode, bodyText)
	}

	decoded, err := decodeAPIResponse[T](body)
	if err != nil {
		return apiResponse[T]{}, false, errb.Wrapf(err, "datago response decode failed")
	}
	if decoded.Header.ResultCode != "" && decoded.Header.ResultCode != "00" {
		return apiResponse[T]{}, false, errb.With("result_code", decoded.Header.ResultCode, "result_msg", decoded.Header.ResultMsg).Errorf("datago remote error provider=%s group=%s operation=%s page=%d result_code=%s result_msg=%s", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, decoded.Header.ResultCode, decoded.Header.ResultMsg)
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

type apiResponse[T any] struct {
	Header apiHeader
	Body   apiBody[T]
}

type apiHeader struct {
	ResultCode string `json:"resultCode"`
	ResultMsg  string `json:"resultMsg"`
}

type apiBody[T any] struct {
	NumOfRows  int
	PageNo     int
	TotalCount int
	Items      []T
}

func decodeAPIResponse[T any](body []byte) (apiResponse[T], error) {
	errb := oops.In("datago_client")
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
		return apiResponse[T]{}, errb.Wrapf(err, "decode datago JSON envelope")
	}
	response := raw.Response
	if response == nil {
		if raw.Header.ResultCode == "" && raw.Header.ResultMsg == "" && raw.Body.PageNo == 0 && raw.Body.NumOfRows == 0 && raw.Body.TotalCount == 0 {
			return apiResponse[T]{}, errb.New("decode datago JSON envelope: response is required")
		}
		response = &raw.responseEnvelope
	}

	items, err := decodeItems[T](response.Body.Items.Item)
	if err != nil {
		return apiResponse[T]{}, errb.Wrapf(err, "decode datago items")
	}
	return apiResponse[T]{
		Header: response.Header,
		Body: apiBody[T]{
			NumOfRows:  response.Body.NumOfRows,
			PageNo:     response.Body.PageNo,
			TotalCount: response.Body.TotalCount,
			Items:      items,
		},
	}, nil
}

func decodeItems[T any](raw json.RawMessage) ([]T, error) {
	errb := oops.In("datago_client")
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	switch trimmed[0] {
	case '[':
		var items []T
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, errb.Wrapf(err, "decode datago item array")
		}
		return items, nil
	case '{':
		var item T
		if err := json.Unmarshal(trimmed, &item); err != nil {
			return nil, errb.Wrapf(err, "decode datago item object")
		}
		return []T{item}, nil
	default:
		return nil, errb.With("item", string(trimmed)).Errorf("unsupported item shape: %s", string(trimmed))
	}
}
