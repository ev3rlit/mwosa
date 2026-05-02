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

	"github.com/samber/oops"
)

type Client struct {
	serviceKey string
	baseURL    string
	httpClient *http.Client
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
		serviceKey: config.ServiceKey,
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: config.HTTPClient,
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
	result, err := fetchAllPriceInfoPages[ETFPriceInfo](c, ctx, OperationGetETFPriceInfo, query.values(), query.numOfRows())
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
	result, err := fetchAllPriceInfoPages[ETNPriceInfo](c, ctx, OperationGetETNPriceInfo, query.values(), query.numOfRows())
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
	result, err := fetchAllPriceInfoPages[ELWPriceInfo](c, ctx, OperationGetELWPriceInfo, query.values(), query.numOfRows())
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

func fetchAllPriceInfoPages[T any](c *Client, ctx context.Context, operation string, params url.Values, numOfRows int) (priceInfoResult[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
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

func fetchPage[T any](c *Client, ctx context.Context, operation string, params url.Values, pageNo int, numOfRows int) (apiResponse[T], error) {
	errb := oops.In("datago_client").With(
		"provider", ProviderDataGo,
		"group", GroupSecuritiesProductPrice,
		"operation", operation,
		"page", pageNo,
	)

	endpoint := fmt.Sprintf("%s/%s", c.baseURL, operation)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return apiResponse[T]{}, errb.Wrapf(err, "datago request build failed")
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
		return apiResponse[T]{}, errb.Wrapf(err, "datago request build failed")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse[T]{}, errb.Wrapf(err, "datago remote request failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse[T]{}, errb.With("status", resp.StatusCode).Wrapf(err, "datago remote response read failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyText := strings.TrimSpace(string(body))
		return apiResponse[T]{}, errb.With("status", resp.StatusCode, "body", bodyText).Errorf("datago remote error provider=%s group=%s operation=%s page=%d status=%d body=%s", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, resp.StatusCode, bodyText)
	}

	decoded, err := decodeAPIResponse[T](body)
	if err != nil {
		return apiResponse[T]{}, errb.Wrapf(err, "datago response decode failed")
	}
	if decoded.Header.ResultCode != "" && decoded.Header.ResultCode != "00" {
		return apiResponse[T]{}, errb.With("result_code", decoded.Header.ResultCode, "result_msg", decoded.Header.ResultMsg).Errorf("datago remote error provider=%s group=%s operation=%s page=%d result_code=%s result_msg=%s", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, decoded.Header.ResultCode, decoded.Header.ResultMsg)
	}
	return decoded, nil
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
