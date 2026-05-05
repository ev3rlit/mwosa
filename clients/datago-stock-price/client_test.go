package stockprice

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGetStockPriceInfoBuildsOpenAPIQueryAndParsesTypedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getStockPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "100")
		assertQuery(t, r, "basDt", "20240415")
		assertQuery(t, r, "likeSrtnCd", "005930")
		assertQuery(t, r, "mrktCls", "KOSPI")
		assertQuery(t, r, "beginTrPrc", "1000000000")
		fmt.Fprint(w, `{
			"response": {
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
							"clpr": 82200,
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
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetStockPriceInfo(context.Background(), StockPriceInfoQuery{
		BasDt:      "20240415",
		LikeSrtnCd: "005930",
		NumOfRows:  100,
		MrktCls:    "KOSPI",
		BeginTrPrc: "1000000000",
	})
	if err != nil {
		t.Fatalf("get stock price info: %v", err)
	}
	if result.TotalCount != 1 {
		t.Fatalf("total count = %d, want 1", result.TotalCount)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.SrtnCd != "005930" || item.Clpr != "82200" || item.MrktCtg != "KOSPI" {
		t.Fatalf("unexpected parsed stock item: %+v", item)
	}
	if item.Fields()["lstgStCnt"] != "5969782550" {
		t.Fatalf("raw lstgStCnt field = %q, want 5969782550", item.Fields()["lstgStCnt"])
	}
}

func TestStockPriceInfoQueryWithInstrumentSearch(t *testing.T) {
	codeQuery := StockPriceInfoQuery{}.WithInstrumentSearch("005930")
	if got := codeQuery.values().Get("likeSrtnCd"); got != "005930" {
		t.Fatalf("likeSrtnCd = %q, want 005930", got)
	}

	isinQuery := StockPriceInfoQuery{}.WithInstrumentSearch("KR7005930003")
	if got := isinQuery.values().Get("likeIsinCd"); got != "KR7005930003" {
		t.Fatalf("likeIsinCd = %q, want KR7005930003", got)
	}

	nameQuery := StockPriceInfoQuery{}.WithInstrumentSearch("Samsung")
	if got := nameQuery.values().Get("likeItmsNm"); got != "Samsung" {
		t.Fatalf("likeItmsNm = %q, want Samsung", got)
	}
}

func TestGetAllStockPriceInfoUsesWorkersForRemainingPages(t *testing.T) {
	seenPages := make(map[string]bool)
	var seenMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getStockPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenMu.Lock()
		seenPages[pageNo] = true
		seenMu.Unlock()
		assertCommonQuery(t, r, pageNo, "1")
		assertQuery(t, r, "basDt", "20240415")

		switch pageNo {
		case "1":
			fmt.Fprint(w, stockPageJSON(1, 3, "005930", "Samsung Electronics"))
		case "2":
			fmt.Fprint(w, stockPageJSON(2, 3, "000660", "SK hynix"))
		case "3":
			fmt.Fprint(w, stockPageJSON(3, 3, "373220", "LG Energy Solution"))
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetAllStockPriceInfo(context.Background(), StockPriceInfoQuery{
		BasDt:     "20240415",
		NumOfRows: 1,
		Workers:   2,
	})
	if err != nil {
		t.Fatalf("get all stock price info: %v", err)
	}
	for _, pageNo := range []string{"1", "2", "3"} {
		if !seenPages[pageNo] {
			t.Fatalf("page %s was not fetched; seen=%v", pageNo, seenPages)
		}
	}
	if result.NumOfRows != 1 || result.PageNo != 1 || result.TotalCount != 3 {
		t.Fatalf("pagination = pageNo:%d numOfRows:%d totalCount:%d, want 1/1/3", result.PageNo, result.NumOfRows, result.TotalCount)
	}
	if len(result.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(result.Items))
	}
	if result.Items[0].SrtnCd != "005930" || result.Items[1].SrtnCd != "000660" || result.Items[2].SrtnCd != "373220" {
		t.Fatalf("unexpected items: %+v", result.Items)
	}
}

func TestRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.GetStockPriceInfo(context.Background(), StockPriceInfoQuery{BasDt: "20240415"})
	if err == nil {
		t.Fatal("get stock price info error = nil, want remote error")
	}
	for _, want := range []string{"provider=datago", "group=stockPrice", "operation=getStockPriceInfo", "status=502"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func stockPageJSON(pageNo int, totalCount int, code string, name string) string {
	return fmt.Sprintf(`{
		"response": {
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 1,
				"pageNo": %d,
				"totalCount": %d,
				"items": {"item": [
					{"basDt": "20240415", "srtnCd": %q, "itmsNm": %q, "clpr": "1000"}
				]}
			}
		}
	}`, pageNo, totalCount, code, name)
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := New(Config{
		ServiceKey:       "test-key",
		BaseURL:          baseURL,
		RetryMaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("new datago stock price client: %v", err)
	}
	return client
}

func assertCommonQuery(t *testing.T, r *http.Request, pageNo string, numOfRows string) {
	t.Helper()
	if got := r.URL.Query().Get("serviceKey"); got != "test-key" {
		t.Fatalf("serviceKey = %q, want test-key", got)
	}
	if got := r.URL.Query().Get("resultType"); got != "json" {
		t.Fatalf("resultType = %q, want json", got)
	}
	if got := r.URL.Query().Get("pageNo"); got != pageNo {
		t.Fatalf("pageNo = %q, want %s", got, pageNo)
	}
	if got := r.URL.Query().Get("numOfRows"); got != numOfRows {
		t.Fatalf("numOfRows = %q, want %s", got, numOfRows)
	}
}

func assertQuery(t *testing.T, r *http.Request, key string, want string) {
	t.Helper()
	if got := r.URL.Query().Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
