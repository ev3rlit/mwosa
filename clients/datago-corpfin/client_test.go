package corpfin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetSummaryFinancialStatementsBuildsQueryAndParsesTypedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getSummFinaStat_V2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "100")
		assertQuery(t, r, "crno", "1746110000741")
		assertQuery(t, r, "bizYear", "2019")
		fmt.Fprint(w, `{
			"response": {
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 100,
					"pageNo": 1,
					"totalCount": 1,
					"items": {
						"item": {
							"basDt": "20200101",
							"bizYear": "2019",
							"crno": "1746110000741",
							"curCd": "KRW",
							"enpSaleAmt": 1000,
							"enpBzopPft": "200",
							"enpTastAmt": "5000",
							"enpTdbtAmt": "2500",
							"enpTcptAmt": "2500",
							"fnclDcd": "01",
							"fnclDcdNm": "연결"
						}
					}
				}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetSummaryFinancialStatements(context.Background(), Query{
		Crno:    "1746110000741",
		BizYear: "2019",
	})
	if err != nil {
		t.Fatalf("get summary financial statements: %v", err)
	}
	if result.TotalCount != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %+v, want one item", result)
	}
	item := result.Items[0]
	if item.EnpSaleAmt != "1000" || item.EnpBzopPft != "200" || item.Fields()["fnclDcdNm"] != "연결" {
		t.Fatalf("unexpected parsed summary item: %+v", item)
	}
}

func TestGetAllBalanceSheetsCollectsAllPages(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getBs_V2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		pageNo := r.URL.Query().Get("pageNo")
		seenPages = append(seenPages, pageNo)
		assertQuery(t, r, "numOfRows", "1000")
		switch pageNo {
		case "1":
			fmt.Fprint(w, `{
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 1000,
					"pageNo": 1,
					"totalCount": 1001,
					"items": {"item": [
						{"basDt": "20200101", "bizYear": "2019", "crno": "1746110000741", "acitId": "ifrs_Assets", "acitNm": "Assets", "crtmAcitAmt": "5000"}
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
						{"basDt": "20200101", "bizYear": "2019", "crno": "1746110000741", "acitId": "ifrs_Liabilities", "acitNm": "Liabilities", "crtmAcitAmt": "2500"}
					]}
				}
			}`)
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetAllBalanceSheets(context.Background(), Query{
		Crno:    "1746110000741",
		BizYear: "2019",
	})
	if err != nil {
		t.Fatalf("get all balance sheets: %v", err)
	}
	if strings.Join(seenPages, ",") != "1,2" {
		t.Fatalf("seen pages = %v, want [1 2]", seenPages)
	}
	if len(result.Items) != 2 || result.Items[1].AcitID != "ifrs_Liabilities" {
		t.Fatalf("items = %+v, want page-order balance sheet rows", result.Items)
	}
}

func TestGetIncomeStatementsDecodesArrayItemsForRequestedPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getIncoStat_V2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "2", "1")
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 1,
				"pageNo": 2,
				"totalCount": 2,
				"items": {"item": [
					{"basDt": "20200101", "bizYear": "2019", "crno": "1746110000741", "acitId": "ifrs_Revenue", "acitNm": "Revenue", "thqrAcitAmt": 300, "crtmAcitAmt": "1000"}
				]}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetIncomeStatements(context.Background(), Query{
		PageNo:    2,
		NumOfRows: 1,
		Crno:      "1746110000741",
		BizYear:   "2019",
	})
	if err != nil {
		t.Fatalf("get income statements: %v", err)
	}
	if result.PageNo != 2 || result.NumOfRows != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %+v, want requested page", result)
	}
	if result.Items[0].ThqrAcitAmt != "300" || result.Items[0].CrtmAcitAmt != "1000" {
		t.Fatalf("unexpected income row: %+v", result.Items[0])
	}
}

func TestGetSummaryFinancialStatementsRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.GetSummaryFinancialStatements(context.Background(), Query{})
	if err == nil {
		t.Fatal("get summary financial statements error = nil, want remote error")
	}
	for _, want := range []string{"provider=datago", "group=corporateFinance", "operation=getSummFinaStat_V2", "status=502"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestGetSummaryFinancialStatementsRetriesTransientStatus(t *testing.T) {
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
				"items": {"item": {"crno": "1746110000741", "bizYear": "2019", "enpSaleAmt": "1000"}}
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
		t.Fatalf("new datago corporate finance client: %v", err)
	}
	result, err := client.GetSummaryFinancialStatements(context.Background(), Query{})
	if err != nil {
		t.Fatalf("get summary financial statements: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(result.Items) != 1 || result.Items[0].Crno != "1746110000741" {
		t.Fatalf("items = %+v, want retried summary row", result.Items)
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
		t.Fatalf("new datago corporate finance client: %v", err)
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
