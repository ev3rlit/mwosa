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
		assertCommonQuery(t, r, "1")
		if got := r.URL.Query().Get("resultType"); got != "json" {
			t.Fatalf("resultType = %q, want json", got)
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

	p := newTestProvider(t, server.URL, 100)
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

func TestFetchDailyBarsDecodesArrayItemsAndPaginates(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETNPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenPages = append(seenPages, pageNo)
		assertCommonQuery(t, r, pageNo)

		switch pageNo {
		case "1":
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
		case "2":
			fmt.Fprint(w, `{
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 1,
					"pageNo": 2,
					"totalCount": 2,
					"items": {"item": [
						{"basDt": "20240416", "srtnCd": "580001", "itmsNm": "ETN A", "clpr": "1005"}
					]}
				}
			}`)
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL, 1)
	result, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETN,
		Symbol:       "580001",
	})
	if err != nil {
		t.Fatalf("fetch daily bars: %v", err)
	}

	if strings.Join(seenPages, ",") != "1,2" {
		t.Fatalf("seen pages = %v, want [1 2]", seenPages)
	}
	if len(result.Bars) != 2 {
		t.Fatalf("bars len = %d, want 2", len(result.Bars))
	}
	if result.Bars[1].Close != "1005" {
		t.Fatalf("second close = %q, want 1005", result.Bars[1].Close)
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

	p := newTestProvider(t, server.URL, 100)
	result, err := p.SearchInstruments(context.Background(), instrumentInput("missing"))
	if err != nil {
		t.Fatalf("search instruments: %v", err)
	}
	if len(result.Instruments) != 0 {
		t.Fatalf("instruments len = %d, want 0", len(result.Instruments))
	}
	if len(result.Operations) != 2 {
		t.Fatalf("operations len = %d, want ETF/ETN defaults", len(result.Operations))
	}
}

func TestRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	p := newTestProvider(t, server.URL, 100)
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
	p := NewWithClient(&Client{})
	_, err := p.FetchDailyBars(context.Background(), dailybar.FetchInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeStock,
		Symbol:       "005930",
	})
	if err == nil {
		t.Fatal("fetch stock error = nil, want unsupported error")
	}
	var unsupported *provider.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error type = %T, want UnsupportedError: %v", err, err)
	}
	for _, want := range []string{"provider=datago", "group=securitiesProductPrice", "security_type=stock", "symbol=005930"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestRouterReportsUnsupportedQuotePath(t *testing.T) {
	p := NewWithClient(&Client{})
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

func newTestProvider(t *testing.T, baseURL string, numOfRows int) *Provider {
	t.Helper()
	p, err := New(Config{
		ServiceKey: "test-key",
		BaseURL:    baseURL,
		NumOfRows:  numOfRows,
	})
	if err != nil {
		t.Fatalf("new datago provider: %v", err)
	}
	return p
}

func assertCommonQuery(t *testing.T, r *http.Request, pageNo string) {
	t.Helper()
	if got := r.URL.Query().Get("serviceKey"); got != "test-key" {
		t.Fatalf("serviceKey = %q, want test-key", got)
	}
	if got := r.URL.Query().Get("pageNo"); got != pageNo {
		t.Fatalf("pageNo = %q, want %s", got, pageNo)
	}
}

func instrumentInput(query string) instrument.SearchInput {
	return instrument.SearchInput{
		Market: provider.MarketKRX,
		Query:  query,
	}
}
