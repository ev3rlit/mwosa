package corpfin

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
		"group", GroupCorporateFinance,
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

type SummaryFinancialStatementResult struct {
	Items      []SummaryFinancialStatement
	NumOfRows  int
	PageNo     int
	TotalCount int
}

type BalanceSheetResult struct {
	Items      []BalanceSheetItem
	NumOfRows  int
	PageNo     int
	TotalCount int
}

type IncomeStatementResult struct {
	Items      []IncomeStatementItem
	NumOfRows  int
	PageNo     int
	TotalCount int
}

func (c *Client) GetSummaryFinancialStatements(ctx context.Context, query Query) (SummaryFinancialStatementResult, error) {
	result, err := fetchPage[SummaryFinancialStatement](c, ctx, OperationGetSummFinaStatV2, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return SummaryFinancialStatementResult{}, err
	}
	return SummaryFinancialStatementResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetAllSummaryFinancialStatements(ctx context.Context, query Query) (SummaryFinancialStatementResult, error) {
	query = query.forAllPages()
	result, err := fetchAllPages[SummaryFinancialStatement](c, ctx, OperationGetSummFinaStatV2, query.values(), query.numOfRows())
	if err != nil {
		return SummaryFinancialStatementResult{}, err
	}
	return SummaryFinancialStatementResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetBalanceSheets(ctx context.Context, query Query) (BalanceSheetResult, error) {
	result, err := fetchPage[BalanceSheetItem](c, ctx, OperationGetBalanceSheetV2, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return BalanceSheetResult{}, err
	}
	return BalanceSheetResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetAllBalanceSheets(ctx context.Context, query Query) (BalanceSheetResult, error) {
	query = query.forAllPages()
	result, err := fetchAllPages[BalanceSheetItem](c, ctx, OperationGetBalanceSheetV2, query.values(), query.numOfRows())
	if err != nil {
		return BalanceSheetResult{}, err
	}
	return BalanceSheetResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetIncomeStatements(ctx context.Context, query Query) (IncomeStatementResult, error) {
	result, err := fetchPage[IncomeStatementItem](c, ctx, OperationGetIncomeStatementV2, query.values(), query.pageNo(), query.numOfRows())
	if err != nil {
		return IncomeStatementResult{}, err
	}
	return IncomeStatementResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

func (c *Client) GetAllIncomeStatements(ctx context.Context, query Query) (IncomeStatementResult, error) {
	query = query.forAllPages()
	result, err := fetchAllPages[IncomeStatementItem](c, ctx, OperationGetIncomeStatementV2, query.values(), query.numOfRows())
	if err != nil {
		return IncomeStatementResult{}, err
	}
	return IncomeStatementResult{Items: result.Items, NumOfRows: result.NumOfRows, PageNo: result.PageNo, TotalCount: result.TotalCount}, nil
}

type result[T any] struct {
	Items      []T
	NumOfRows  int
	PageNo     int
	TotalCount int
}

func fetchAllPages[T any](c *Client, ctx context.Context, operation string, params url.Values, numOfRows int) (result[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupCorporateFinance,
		"operation", operation,
	)
	first, err := fetchPage[T](c, ctx, operation, params, 1, numOfRows)
	if err != nil {
		return result[T]{}, errb.With("page", 1).Wrapf(err, "fetch datago corporate finance first page")
	}
	items := append([]T(nil), first.Items...)
	effectiveNumOfRows := first.NumOfRows
	if effectiveNumOfRows <= 0 {
		effectiveNumOfRows = numOfRows
	}
	pageCount := pageCount(first.TotalCount, effectiveNumOfRows)
	for pageNo := 2; pageNo <= pageCount; pageNo++ {
		next, err := fetchPage[T](c, ctx, operation, params, pageNo, numOfRows)
		if err != nil {
			return result[T]{}, errb.With("page", pageNo).Wrapf(err, "fetch datago corporate finance page")
		}
		items = append(items, next.Items...)
	}
	return result[T]{
		Items:      items,
		NumOfRows:  effectiveNumOfRows,
		PageNo:     1,
		TotalCount: first.TotalCount,
	}, nil
}

func fetchPage[T any](c *Client, ctx context.Context, operation string, params url.Values, pageNo int, numOfRows int) (result[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupCorporateFinance,
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
			return result[T]{Items: decoded.Body.Items, NumOfRows: decoded.Body.NumOfRows, PageNo: decoded.Body.PageNo, TotalCount: decoded.Body.TotalCount}, nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts {
			return result[T]{}, err
		}
		if err := sleepBeforeRetry(ctx, retryDelay(c, attempt)); err != nil {
			return result[T]{}, errb.With("attempt", attempt).Wrap(oops.Join(lastErr, err))
		}
	}
	return result[T]{}, errb.Wrap(lastErr)
}

func fetchPageOnce[T any](c *Client, ctx context.Context, operation string, params url.Values, pageNo int, numOfRows int) (apiResponse[T], bool, error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupCorporateFinance,
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
		return apiResponse[T]{}, shouldRetryStatus(resp.StatusCode), errb.With("status", resp.StatusCode, "body", bodyText).Errorf("datago remote error provider=%s group=%s operation=%s page=%d status=%d body=%s", ProviderDataGo, GroupCorporateFinance, operation, pageNo, resp.StatusCode, bodyText)
	}

	decoded, err := decodeAPIResponse[T](body)
	if err != nil {
		return apiResponse[T]{}, false, errb.Wrapf(err, "datago response decode failed")
	}
	if decoded.Header.ResultCode != "" && decoded.Header.ResultCode != "00" {
		return apiResponse[T]{}, false, errb.With("result_code", decoded.Header.ResultCode, "result_msg", decoded.Header.ResultMsg).Errorf("datago remote error provider=%s group=%s operation=%s page=%d result_code=%s result_msg=%s", ProviderDataGo, GroupCorporateFinance, operation, pageNo, decoded.Header.ResultCode, decoded.Header.ResultMsg)
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
