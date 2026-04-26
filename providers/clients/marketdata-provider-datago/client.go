package datagoclient

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
)

type Client struct {
	serviceKey string
	baseURL    string
	httpClient *http.Client
	numOfRows  int
}

func New(config Config) (*Client, error) {
	config = config.withDefaults()
	if strings.TrimSpace(config.ServiceKey) == "" {
		return nil, fmt.Errorf("datago client config: serviceKey is required provider=%s group=%s", ProviderDataGo, GroupSecuritiesProductPrice)
	}
	if _, err := url.ParseRequestURI(config.BaseURL); err != nil {
		return nil, fmt.Errorf("datago client config: invalid baseURL provider=%s group=%s baseURL=%q: %w", ProviderDataGo, GroupSecuritiesProductPrice, config.BaseURL, err)
	}
	return &Client{
		serviceKey: config.ServiceKey,
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		httpClient: config.HTTPClient,
		numOfRows:  config.NumOfRows,
	}, nil
}

type PriceQuery struct {
	Operation string
	Params    url.Values
	Limit     int
}

type PriceResult struct {
	Items      []PriceItem
	TotalCount int
}

func (c *Client) FetchPrices(ctx context.Context, query PriceQuery) (PriceResult, error) {
	if query.Operation == "" {
		return PriceResult{}, fmt.Errorf("datago fetch prices: operation is required provider=%s group=%s", ProviderDataGo, GroupSecuritiesProductPrice)
	}

	allItems := make([]PriceItem, 0)
	totalCount := 0
	pageNo := 1

	for {
		response, err := c.fetchPage(ctx, query.Operation, query.Params, pageNo)
		if err != nil {
			return PriceResult{}, err
		}
		totalCount = response.Body.TotalCount
		allItems = append(allItems, response.Body.Items...)

		if query.Limit > 0 && len(allItems) >= query.Limit {
			allItems = allItems[:query.Limit]
			break
		}
		if totalCount == 0 || len(allItems) >= totalCount || len(response.Body.Items) == 0 {
			break
		}
		pageNo++
	}

	return PriceResult{Items: allItems, TotalCount: totalCount}, nil
}

func (c *Client) fetchPage(ctx context.Context, operation string, params url.Values, pageNo int) (apiResponse, error) {
	endpoint := fmt.Sprintf("%s/%s", c.baseURL, operation)
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return apiResponse{}, fmt.Errorf("datago request build failed provider=%s group=%s operation=%s: %w", ProviderDataGo, GroupSecuritiesProductPrice, operation, err)
	}

	values := cloneValues(params)
	values.Set("serviceKey", c.serviceKey)
	values.Set("resultType", "json")
	values.Set("numOfRows", strconv.Itoa(c.numOfRows))
	values.Set("pageNo", strconv.Itoa(pageNo))
	reqURL.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return apiResponse{}, fmt.Errorf("datago request build failed provider=%s group=%s operation=%s: %w", ProviderDataGo, GroupSecuritiesProductPrice, operation, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse{}, fmt.Errorf("datago remote request failed provider=%s group=%s operation=%s page=%d: %w", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, fmt.Errorf("datago remote response read failed provider=%s group=%s operation=%s page=%d status=%d: %w", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, resp.StatusCode, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiResponse{}, fmt.Errorf("datago remote error provider=%s group=%s operation=%s page=%d status=%d body=%s", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	decoded, err := decodeAPIResponse(body)
	if err != nil {
		return apiResponse{}, fmt.Errorf("datago response decode failed provider=%s group=%s operation=%s page=%d: %w", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, err)
	}
	if decoded.Header.ResultCode != "" && decoded.Header.ResultCode != "00" {
		return apiResponse{}, fmt.Errorf("datago remote error provider=%s group=%s operation=%s page=%d result_code=%s result_msg=%s", ProviderDataGo, GroupSecuritiesProductPrice, operation, pageNo, decoded.Header.ResultCode, decoded.Header.ResultMsg)
	}
	return decoded, nil
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
	ResultCode string
	ResultMsg  string
}

type apiBody struct {
	NumOfRows  int
	PageNo     int
	TotalCount int
	Items      []PriceItem
}

func decodeAPIResponse(body []byte) (apiResponse, error) {
	var raw struct {
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

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return apiResponse{}, err
	}

	items, err := decodeItems(raw.Body.Items.Item)
	if err != nil {
		return apiResponse{}, err
	}
	return apiResponse{
		Header: raw.Header,
		Body: apiBody{
			NumOfRows:  raw.Body.NumOfRows,
			PageNo:     raw.Body.PageNo,
			TotalCount: raw.Body.TotalCount,
			Items:      items,
		},
	}, nil
}

type PriceItem map[string]string

func (p *PriceItem) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}

	item := make(PriceItem, len(raw))
	for key, value := range raw {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			item[key] = typed
		case json.Number:
			item[key] = typed.String()
		case bool:
			item[key] = strconv.FormatBool(typed)
		default:
			item[key] = fmt.Sprint(typed)
		}
	}
	*p = item
	return nil
}

func decodeItems(raw json.RawMessage) ([]PriceItem, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	switch trimmed[0] {
	case '[':
		var items []PriceItem
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, err
		}
		return items, nil
	case '{':
		var item PriceItem
		if err := json.Unmarshal(trimmed, &item); err != nil {
			return nil, err
		}
		return []PriceItem{item}, nil
	default:
		return nil, fmt.Errorf("unsupported item shape: %s", string(trimmed))
	}
}
