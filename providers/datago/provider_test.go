package datago

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/instrument"
	"github.com/ev3rlit/mwosa/providers/core/quote"
)

func TestFetchDailyBarsDecodesSingleObjectItem(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "1000")
		if got := r.URL.Query().Get("resultType"); got != "json" {
			t.Fatalf("resultType = %q, want json", got)
		}
		if got := r.URL.Query().Get("likeSrtnCd"); got != "069500" {
			t.Fatalf("likeSrtnCd = %q, want 069500", got)
		}
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 1,
				"items": {
					"item": {
						"basDt": "20240415",
						"srtnCd": "069500",
						"isinCd": "KR7069500007",
						"itmsNm": "KODEX 200",
						"clpr": 35120,
						"vs": "-15",
						"fltRt": "-0.04",
						"mkp": "35100",
						"hipr": "35200",
						"lopr": "35000",
						"trqu": "123456",
						"trPrc": "4321000",
						"mrktTotAmt": "1000000000",
						"nav": "35155.1"
					}
				}
			}
		}`)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "069500",
		From:         "20240415",
		To:           "20240415",
	})
	if err != nil {
		t.Fatalf("fetch daily bars: %v", err)
	}

	if len(result.Bars) != 1 {
		t.Fatalf("bars len = %d, want 1", len(result.Bars))
	}
	bar := result.Bars[0]
	if bar.Provider != provider.ProviderDataGo || bar.Group != provider.GroupSecuritiesProductPrice {
		t.Fatalf("unexpected provenance: provider=%s group=%s", bar.Provider, bar.Group)
	}
	if bar.Operation != provider.OperationGetETFPriceInfo {
		t.Fatalf("operation = %s, want %s", bar.Operation, provider.OperationGetETFPriceInfo)
	}
	if bar.TradingDate != "2024-04-15" {
		t.Fatalf("trading date = %q, want 2024-04-15", bar.TradingDate)
	}
	if bar.Close != "35120" {
		t.Fatalf("close = %q, want 35120", bar.Close)
	}
	if bar.Extensions["nav"] != "35155.1" {
		t.Fatalf("nav extension = %q, want 35155.1", bar.Extensions["nav"])
	}
}

func TestFetchDailyBarsUsesNameSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "1000")
		if got := r.URL.Query().Get("itmsNm"); got != "KODEX 200" {
			t.Fatalf("itmsNm = %q, want KODEX 200", got)
		}
		if got := r.URL.Query().Get("likeSrtnCd"); got != "" {
			t.Fatalf("likeSrtnCd = %q, want empty", got)
		}
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 1,
				"items": {
					"item": {
						"basDt": "20240415",
						"srtnCd": "069500",
						"itmsNm": "KODEX 200",
						"clpr": "35120"
					}
				}
			}
		}`)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "KODEX 200",
		From:         "20240415",
		To:           "20240415",
	})
	if err != nil {
		t.Fatalf("fetch daily bars: %v", err)
	}
	if len(result.Bars) != 1 || result.Bars[0].Symbol != "069500" {
		t.Fatalf("bars = %+v, want KODEX 200 match", result.Bars)
	}
}

func TestFetchDailyBarsCollectsAllPagesWhenLimitOmitted(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenPages = append(seenPages, pageNo)
		assertCommonQuery(t, r, pageNo, "1000")
		if got := r.URL.Query().Get("beginBasDt"); got != "20240415" {
			t.Fatalf("beginBasDt = %q, want 20240415", got)
		}
		if got := r.URL.Query().Get("endBasDt"); got != "20240417" {
			t.Fatalf("endBasDt = %q, want 20240417", got)
		}
		if got := r.URL.Query().Get("basDt"); got != "" {
			t.Fatalf("basDt = %q, want empty", got)
		}
		switch pageNo {
		case "1":
			fmt.Fprint(w, `{
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 1000,
					"pageNo": 1,
					"totalCount": 1001,
					"items": {"item": [
						{"basDt": "20240415", "srtnCd": "069500", "itmsNm": "KODEX 200", "clpr": "35120"}
					]}
				}
			}`)
		case "2":
			fmt.Fprint(w, `{
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 1000,
					"pageNo": 2,
					"totalCount": 1001,
					"items": {"item": [
						{"basDt": "20240415", "srtnCd": "069501", "itmsNm": "KODEX Next", "clpr": "1000"}
					]}
				}
			}`)
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		From:         "20240415",
		To:           "20240416",
	})
	if err != nil {
		t.Fatalf("fetch daily bars: %v", err)
	}
	if strings.Join(seenPages, ",") != "1,2" {
		t.Fatalf("seen pages = %v, want [1 2]", seenPages)
	}
	if len(result.Bars) != 2 || result.TotalCount != 1001 {
		t.Fatalf("result = %+v, want two fetched bars with totalCount 1001", result)
	}
}

func TestFetchDailyBarsDecodesOnlyRequestedPage(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETNPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenPages = append(seenPages, pageNo)
		assertCommonQuery(t, r, pageNo, "1")
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 1,
				"pageNo": 1,
				"totalCount": 2,
				"items": {"item": [
					{"basDt": "20240415", "srtnCd": "580001", "itmsNm": "ETN A", "clpr": "1000"}
				]}
			}
		}`)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETN,
		Symbol:       "580001",
		Limit:        1,
	})
	if err != nil {
		t.Fatalf("fetch daily bars: %v", err)
	}

	if strings.Join(seenPages, ",") != "1" {
		t.Fatalf("seen pages = %v, want [1]", seenPages)
	}
	if len(result.Bars) != 1 {
		t.Fatalf("bars len = %d, want 1", len(result.Bars))
	}
	if result.Bars[0].Close != "1000" {
		t.Fatalf("close = %q, want 1000", result.Bars[0].Close)
	}
}

func TestSearchInstrumentsReturnsEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 0,
				"items": {}
			}
		}`)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.SearchInstruments(context.Background(), instrumentInput("missing"))
	if err != nil {
		t.Fatalf("search instruments: %v", err)
	}
	if len(result.Instruments) != 0 {
		t.Fatalf("instruments len = %d, want 0", len(result.Instruments))
	}
	if len(result.Operations) != 3 {
		t.Fatalf("operations len = %d, want ETF/ETN/stock defaults", len(result.Operations))
	}
}

func TestFetchStockDailyBarsUsesStockPriceGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getStockPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "1000")
		if got := r.URL.Query().Get("resultType"); got != "json" {
			t.Fatalf("resultType = %q, want json", got)
		}
		if got := r.URL.Query().Get("likeSrtnCd"); got != "005930" {
			t.Fatalf("likeSrtnCd = %q, want 005930", got)
		}
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 1,
				"items": {
					"item": {
						"basDt": "20240415",
						"srtnCd": "005930",
						"isinCd": "KR7005930003",
						"itmsNm": "Samsung Electronics",
						"mrktCtg": "KOSPI",
						"clpr": "82200",
						"vs": "-100",
						"fltRt": "-0.12",
						"mkp": "82400",
						"hipr": "82500",
						"lopr": "81500",
						"trqu": "12345678",
						"trPrc": "1000000000000",
						"lstgStCnt": "5969782550",
						"mrktTotAmt": "490000000000000"
					}
				}
			}
		}`)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	result, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeStock,
		Symbol:       "005930",
		From:         "20240415",
		To:           "20240415",
	})
	if err != nil {
		t.Fatalf("fetch stock daily bars: %v", err)
	}
	if len(result.Bars) != 1 {
		t.Fatalf("bars len = %d, want 1", len(result.Bars))
	}
	bar := result.Bars[0]
	if bar.Group != provider.GroupStockPrice {
		t.Fatalf("group = %s, want %s", bar.Group, provider.GroupStockPrice)
	}
	if bar.Operation != provider.OperationGetStockPriceInfo {
		t.Fatalf("operation = %s, want %s", bar.Operation, provider.OperationGetStockPriceInfo)
	}
	if bar.SecurityType != provider.SecurityTypeStock || bar.Symbol != "005930" || bar.Close != "82200" {
		t.Fatalf("unexpected stock bar: %+v", bar)
	}
	if bar.Extensions["lstgStCnt"] != "5969782550" || bar.Extensions["mrktCtg"] != "KOSPI" {
		t.Fatalf("stock extensions = %+v, want listing count and market category", bar.Extensions)
	}
}

func TestRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	_, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "069500",
	})
	if err == nil {
		t.Fatal("fetch daily bars error = nil, want remote error")
	}
	for _, want := range []string{"provider=datago", "group=securitiesProductPrice", "operation=getETFPriceInfo", "status=502"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestUnsupportedSecurityTypeIsNotHiddenAsEmptySuccess(t *testing.T) {
	p := NewWithClient(nil)
	_, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityType("bond"),
		Symbol:       "KRW-BOND",
	})
	if err == nil {
		t.Fatal("fetch unsupported security type error = nil, want unsupported error")
	}
	var unsupported *provider.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error type = %T, want UnsupportedError: %v", err, err)
	}
	for _, want := range []string{"provider=datago", "group=securitiesProductPrice", "security_type=bond", "symbol=KRW-BOND"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestRouterReportsUnsupportedQuotePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL)
	registry := provider.NewRegistry()
	if err := Register(registry, p); err != nil {
		t.Fatalf("register datago provider: %v", err)
	}

	router := quote.NewRouter(provider.NewRouter(registry))
	_, err := router.RouteQuoteSnapshot(context.Background(), quote.RouteInput{
		ProviderID:   provider.ProviderDataGo,
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "069500",
	})
	if err == nil {
		t.Fatal("route quote error = nil, want no provider")
	}
	if !provider.IsNoProvider(err) {
		t.Fatalf("route quote error = %v, want ErrNoProvider", err)
	}
	if !strings.Contains(err.Error(), "role=quote_snapshot") {
		t.Fatalf("error should include role context: %v", err)
	}
}

func TestProviderRegistersGroupRoleEntries(t *testing.T) {
	p := newTestProvider(t, "http://example.invalid")
	registrations := p.RoleRegistrations()
	if len(registrations) != 4 {
		t.Fatalf("registrations len = %d, want 4", len(registrations))
	}

	roles := make(map[provider.GroupID]map[provider.Role]bool)
	for _, registration := range registrations {
		group := registration.Profile.Group
		if roles[group] == nil {
			roles[group] = map[provider.Role]bool{}
		}
		roles[group][registration.Profile.Role] = true
	}
	for _, group := range []provider.GroupID{provider.GroupSecuritiesProductPrice, provider.GroupStockPrice} {
		for _, role := range []provider.Role{provider.RoleDailyBar, provider.RoleInstrument} {
			if !roles[group][role] {
				t.Fatalf("group %s missing role %s in registrations: %+v", group, role, registrations)
			}
		}
	}

	registry := provider.NewRegistry()
	if err := Register(registry, p); err != nil {
		t.Fatalf("register datago provider: %v", err)
	}
	if len(registry.Entries(provider.RoleDailyBar)) != 2 {
		t.Fatalf("dailybar entries len = %d, want 2", len(registry.Entries(provider.RoleDailyBar)))
	}
	if len(registry.Entries(provider.RoleInstrument)) != 2 {
		t.Fatalf("instrument entries len = %d, want 2", len(registry.Entries(provider.RoleInstrument)))
	}
}

func newTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()
	p, err := New(Config{
		ServiceKey: "test-key",
		BaseURL:    baseURL,
		StockPrice: GroupConfig{
			ServiceKey: "test-key",
			BaseURL:    baseURL,
		},
		RetryMaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("new datago provider: %v", err)
	}
	return p
}

func assertCommonQuery(t *testing.T, r *http.Request, pageNo string, numOfRows string) {
	t.Helper()
	if got := r.URL.Query().Get("serviceKey"); got != "test-key" {
		t.Fatalf("serviceKey = %q, want test-key", got)
	}
	if got := r.URL.Query().Get("pageNo"); got != pageNo {
		t.Fatalf("pageNo = %q, want %s", got, pageNo)
	}
	if got := r.URL.Query().Get("numOfRows"); got != numOfRows {
		t.Fatalf("numOfRows = %q, want %s", got, numOfRows)
	}
}

func instrumentInput(query string) instrument.SearchInput {
	return instrument.SearchInput{
		Market: provider.MarketKRX,
		Query:  query,
	}
}
