package etp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetETFPriceInfoBuildsOpenAPIQueryAndParsesTypedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "100")
		assertQuery(t, r, "basDt", "20240415")
		assertQuery(t, r, "likeSrtnCd", "069500")
		assertQuery(t, r, "beginNav", "35000")
		assertQuery(t, r, "likeBssIdxIdxNm", "KOSPI")
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
							"nPptTotAmt": "6010000000",
							"stLstgCnt": "171000",
							"nav": 35155.1,
							"bssIdxIdxNm": "KOSPI 200",
							"bssIdxClpr": "362.14"
						}
					}
				}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetETFPriceInfo(context.Background(), ETFPriceInfoQuery{
		SecuritiesProductPriceQuery: SecuritiesProductPriceQuery{
			BasDt:      "20240415",
			LikeSrtnCd: "069500",
			NumOfRows:  100,
		},
		BeginNav:        "35000",
		LikeBssIdxIdxNm: "KOSPI",
	})
	if err != nil {
		t.Fatalf("get ETF price info: %v", err)
	}
	if result.TotalCount != 1 {
		t.Fatalf("total count = %d, want 1", result.TotalCount)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.SrtnCd != "069500" || item.Clpr != "35120" || item.Nav != "35155.1" {
		t.Fatalf("unexpected parsed ETF item: %+v", item)
	}
	if item.Fields()["nav"] != "35155.1" {
		t.Fatalf("raw nav field = %q, want 35155.1", item.Fields()["nav"])
	}
}

func TestNewUsesDefaultBaseURL(t *testing.T) {
	client, err := New(Config{ServiceKey: "test-key"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, DefaultBaseURL)
	}
}

func TestSecuritiesProductPriceQueryWithInstrumentSearch(t *testing.T) {
	codeQuery := SecuritiesProductPriceQuery{}.WithInstrumentSearch("069500")
	if got := codeQuery.values().Get("likeSrtnCd"); got != "069500" {
		t.Fatalf("likeSrtnCd = %q, want 069500", got)
	}
	if got := codeQuery.values().Get("likeItmsNm"); got != "" {
		t.Fatalf("likeItmsNm = %q, want empty", got)
	}

	isinQuery := SecuritiesProductPriceQuery{}.WithInstrumentSearch("KR7069500007")
	if got := isinQuery.values().Get("likeIsinCd"); got != "KR7069500007" {
		t.Fatalf("likeIsinCd = %q, want KR7069500007", got)
	}

	nameQuery := SecuritiesProductPriceQuery{}.WithInstrumentSearch("KODEX 200")
	if got := nameQuery.values().Get("likeItmsNm"); got != "KODEX 200" {
		t.Fatalf("likeItmsNm = %q, want KODEX 200", got)
	}
}

func TestSecuritiesProductPriceQueryWithInstrumentLookup(t *testing.T) {
	isinQuery := SecuritiesProductPriceQuery{}.WithInstrumentLookup("KR7069500007")
	if got := isinQuery.values().Get("isinCd"); got != "KR7069500007" {
		t.Fatalf("isinCd = %q, want KR7069500007", got)
	}
	if got := isinQuery.values().Get("likeIsinCd"); got != "" {
		t.Fatalf("likeIsinCd = %q, want empty", got)
	}

	nameQuery := SecuritiesProductPriceQuery{}.WithInstrumentLookup("KODEX 200")
	if got := nameQuery.values().Get("itmsNm"); got != "KODEX 200" {
		t.Fatalf("itmsNm = %q, want KODEX 200", got)
	}
	if got := nameQuery.values().Get("likeItmsNm"); got != "" {
		t.Fatalf("likeItmsNm = %q, want empty", got)
	}
}

func TestGetETFPriceInfoMetadataUsesOneRowProbe(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		seenPages = append(seenPages, r.URL.Query().Get("pageNo"))
		assertQuery(t, r, "pageNo", "1")
		assertQuery(t, r, "numOfRows", "1")
		assertQuery(t, r, "basDt", "20260424")
		fmt.Fprint(w, `{
			"response": {
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 1,
					"pageNo": 1,
					"totalCount": 1095,
					"items": {"item": [
						{"basDt": "20260424", "srtnCd": "069500", "itmsNm": "KODEX 200"}
					]}
				}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	metadata, err := client.GetETFPriceInfoMetadata(context.Background(), ETFPriceInfoQuery{
		SecuritiesProductPriceQuery: SecuritiesProductPriceQuery{
			PageNo:    99,
			NumOfRows: 500,
			BasDt:     "20260424",
		},
	})
	if err != nil {
		t.Fatalf("get ETF price info metadata: %v", err)
	}
	if strings.Join(seenPages, ",") != "1" {
		t.Fatalf("seen pages = %v, want [1]", seenPages)
	}
	if metadata.TotalCount != 1095 || metadata.PageSize != 500 || metadata.PageCount != 3 {
		t.Fatalf("metadata = %+v, want totalCount=1095 pageSize=500 pageCount=3", metadata)
	}
	if metadata.ProbePageNo != 1 || metadata.ProbeNumOfRows != 1 {
		t.Fatalf("probe = pageNo:%d numOfRows:%d, want 1/1", metadata.ProbePageNo, metadata.ProbeNumOfRows)
	}
}

func TestGetAllETFPriceInfoUsesFirstPageAsProbe(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenPages = append(seenPages, pageNo)
		assertQuery(t, r, "numOfRows", "1000")
		assertQuery(t, r, "basDt", "20260424")

		switch pageNo {
		case "1":
			fmt.Fprint(w, `{
				"response": {
					"header": {"resultCode": "00", "resultMsg": "OK"},
					"body": {
						"numOfRows": 1000,
						"pageNo": 1,
						"totalCount": 1001,
						"items": {"item": [
							{"basDt": "20260424", "srtnCd": "069500", "itmsNm": "KODEX 200"}
						]}
					}
				}
			}`)
		case "2":
			fmt.Fprint(w, `{
				"response": {
					"header": {"resultCode": "00", "resultMsg": "OK"},
					"body": {
						"numOfRows": 1000,
						"pageNo": 2,
						"totalCount": 1001,
						"items": {"item": [
							{"basDt": "20260424", "srtnCd": "069501", "itmsNm": "KODEX Next"}
						]}
					}
				}
			}`)
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetAllETFPriceInfo(context.Background(), ETFPriceInfoQuery{
		SecuritiesProductPriceQuery: SecuritiesProductPriceQuery{
			PageNo: 99,
			BasDt:  "20260424",
		},
	})
	if err != nil {
		t.Fatalf("get all ETF price info: %v", err)
	}
	if strings.Join(seenPages, ",") != "1,2" {
		t.Fatalf("seen pages = %v, want [1 2]", seenPages)
	}
	if result.NumOfRows != 1000 || result.PageNo != 1 || result.TotalCount != 1001 {
		t.Fatalf("pagination = pageNo:%d numOfRows:%d totalCount:%d, want 1/1000/1001", result.PageNo, result.NumOfRows, result.TotalCount)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(result.Items))
	}
	if result.Items[0].SrtnCd != "069500" || result.Items[1].SrtnCd != "069501" {
		t.Fatalf("unexpected items: %+v", result.Items)
	}
}

func TestGetAllETFPriceInfoUsesWorkersForRemainingPages(t *testing.T) {
	seenPages := make(map[string]bool)
	var seenMu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenMu.Lock()
		seenPages[pageNo] = true
		seenMu.Unlock()
		assertQuery(t, r, "numOfRows", "1000")
		assertQuery(t, r, "beginBasDt", "20260423")
		assertQuery(t, r, "endBasDt", "20260424")

		switch pageNo {
		case "1":
			fmt.Fprint(w, `{
				"response": {
					"header": {"resultCode": "00", "resultMsg": "OK"},
					"body": {
						"numOfRows": 1000,
						"pageNo": 1,
						"totalCount": 2001,
						"items": {"item": [
							{"basDt": "20260423", "srtnCd": "069500", "itmsNm": "KODEX 200"}
						]}
					}
				}
			}`)
		case "2":
			fmt.Fprint(w, `{
				"response": {
					"header": {"resultCode": "00", "resultMsg": "OK"},
					"body": {
						"numOfRows": 1000,
						"pageNo": 2,
						"totalCount": 2001,
						"items": {"item": [
							{"basDt": "20260424", "srtnCd": "069501", "itmsNm": "KODEX Next"}
						]}
					}
				}
			}`)
		case "3":
			fmt.Fprint(w, `{
				"response": {
					"header": {"resultCode": "00", "resultMsg": "OK"},
					"body": {
						"numOfRows": 1000,
						"pageNo": 3,
						"totalCount": 2001,
						"items": {"item": [
							{"basDt": "20260424", "srtnCd": "069502", "itmsNm": "KODEX Last"}
						]}
					}
				}
			}`)
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetAllETFPriceInfo(context.Background(), ETFPriceInfoQuery{
		SecuritiesProductPriceQuery: SecuritiesProductPriceQuery{
			BeginBasDt: "20260423",
			EndBasDt:   "20260424",
			Workers:    2,
		},
	})
	if err != nil {
		t.Fatalf("get all ETF price info: %v", err)
	}
	seenMu.Lock()
	defer seenMu.Unlock()
	if !seenPages["1"] || !seenPages["2"] || !seenPages["3"] {
		t.Fatalf("seen pages = %v, want pages 1, 2, and 3", seenPages)
	}
	if len(result.Items) != 3 {
		t.Fatalf("items len = %d, want 3", len(result.Items))
	}
	if got := []string{result.Items[0].SrtnCd, result.Items[1].SrtnCd, result.Items[2].SrtnCd}; strings.Join(got, ",") != "069500,069501,069502" {
		t.Fatalf("items order = %v, want page order", got)
	}
}

func TestGetETNPriceInfoDecodesArrayItemsForRequestedPage(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETNPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenPages = append(seenPages, pageNo)
		assertCommonQuery(t, r, "2", "1")
		assertQuery(t, r, "beginIndcVal", "990")
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 1,
				"pageNo": 2,
				"totalCount": 2,
				"items": {"item": [
					{"basDt": "20240416", "srtnCd": "580001", "itmsNm": "ETN A", "clpr": "1005", "indcVal": 1012.25}
				]}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetETNPriceInfo(context.Background(), ETNPriceInfoQuery{
		SecuritiesProductPriceQuery: SecuritiesProductPriceQuery{
			PageNo:    2,
			NumOfRows: 1,
		},
		BeginIndcVal: "990",
	})
	if err != nil {
		t.Fatalf("get ETN price info: %v", err)
	}
	if strings.Join(seenPages, ",") != "2" {
		t.Fatalf("seen pages = %v, want [2]", seenPages)
	}
	if result.PageNo != 2 || result.NumOfRows != 1 || result.TotalCount != 2 {
		t.Fatalf("pagination = pageNo:%d numOfRows:%d totalCount:%d, want 2/1/2", result.PageNo, result.NumOfRows, result.TotalCount)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	if result.Items[0].IndcVal != "1012.25" {
		t.Fatalf("indcVal = %q, want 1012.25", result.Items[0].IndcVal)
	}
}

func TestGetELWPriceInfoUsesELWEndpointAndParsesELWFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getELWPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "100")
		assertQuery(t, r, "likeUdasAstNm", "Samsung")
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 1,
				"items": {
					"item": {
						"basDt": "20240415",
						"srtnCd": "57J123",
						"isinCd": "KRA57J123000",
						"itmsNm": "Samsung Call",
						"clpr": "125",
						"lstgScrtCnt": "1000000",
						"udasAstNm": "Samsung",
						"udasAstClpr": 85000
					}
				}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetELWPriceInfo(context.Background(), ELWPriceInfoQuery{
		SecuritiesProductPriceQuery: SecuritiesProductPriceQuery{
			NumOfRows: 100,
		},
		LikeUdasAstNm: "Samsung",
	})
	if err != nil {
		t.Fatalf("get ELW price info: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.SrtnCd != "57J123" || item.UdasAstNm != "Samsung" || item.UdasAstClpr != "85000" {
		t.Fatalf("unexpected parsed ELW item: %+v", item)
	}
}

func TestGetETFPriceInfoReturnsEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCommonQuery(t, r, "1", "100")
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

	client := newTestClient(t, server.URL)
	result, err := client.GetETFPriceInfo(context.Background(), ETFPriceInfoQuery{})
	if err != nil {
		t.Fatalf("get ETF price info: %v", err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("items len = %d, want 0", len(result.Items))
	}
}

func TestGetETFPriceInfoRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.GetETFPriceInfo(context.Background(), ETFPriceInfoQuery{})
	if err == nil {
		t.Fatal("get ETF price info error = nil, want remote error")
	}
	for _, want := range []string{"provider=datago", "group=securitiesProductPrice", "operation=getETFPriceInfo", "status=502"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestGetETFPriceInfoRetriesTransientStatus(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "temporary upstream down", http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 1,
				"items": {"item": {"basDt": "20240415", "srtnCd": "069500", "itmsNm": "KODEX 200"}}
			}
		}`)
	}))
	defer server.Close()

	client, err := New(Config{
		ServiceKey:       "test-key",
		BaseURL:          server.URL,
		RetryMaxAttempts: 2,
		RetryInitialWait: time.Millisecond,
		RetryMaxWait:     time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new datago client: %v", err)
	}
	result, err := client.GetETFPriceInfo(context.Background(), ETFPriceInfoQuery{})
	if err != nil {
		t.Fatalf("get ETF price info: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(result.Items) != 1 || result.Items[0].SrtnCd != "069500" {
		t.Fatalf("items = %+v, want retried KODEX 200 row", result.Items)
	}
}

func TestGetETFPriceInfoResultCodeErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{
			"header": {"resultCode": "99", "resultMsg": "quota exceeded"},
			"body": {
				"numOfRows": 0,
				"pageNo": 1,
				"totalCount": 0,
				"items": {}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.GetETFPriceInfo(context.Background(), ETFPriceInfoQuery{})
	if err == nil {
		t.Fatal("get ETF price info error = nil, want provider error")
	}
	for _, want := range []string{"provider=datago", "group=securitiesProductPrice", "operation=getETFPriceInfo", "result_code=99", "result_msg=quota exceeded"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	client, err := New(Config{
		ServiceKey:       "test-key",
		BaseURL:          baseURL,
		RetryMaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("new datago client: %v", err)
	}
	return client
}

func assertCommonQuery(t *testing.T, r *http.Request, pageNo string, numOfRows string) {
	t.Helper()
	assertQuery(t, r, "serviceKey", "test-key")
	assertQuery(t, r, "pageNo", pageNo)
	assertQuery(t, r, "resultType", "json")
	assertQuery(t, r, "numOfRows", numOfRows)
}

func assertQuery(t *testing.T, r *http.Request, key string, want string) {
	t.Helper()
	if got := r.URL.Query().Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
