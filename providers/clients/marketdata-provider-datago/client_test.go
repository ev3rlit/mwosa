package datagoclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPricesDecodesSingleObjectItem(t *testing.T) {
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
						"itmsNm": "KODEX 200",
						"clpr": 35120,
						"nav": "35155.1"
					}
				}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 100)
	result, err := client.FetchPrices(context.Background(), PriceQuery{
		Operation: OperationGetETFPriceInfo,
	})
	if err != nil {
		t.Fatalf("fetch prices: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item["clpr"] != "35120" {
		t.Fatalf("clpr = %q, want 35120", item["clpr"])
	}
	if item["nav"] != "35155.1" {
		t.Fatalf("nav = %q, want 35155.1", item["nav"])
	}
}

func TestFetchPricesDecodesArrayItemsAndPaginates(t *testing.T) {
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

	client := newTestClient(t, server.URL, 1)
	result, err := client.FetchPrices(context.Background(), PriceQuery{
		Operation: OperationGetETNPriceInfo,
	})
	if err != nil {
		t.Fatalf("fetch prices: %v", err)
	}
	if strings.Join(seenPages, ",") != "1,2" {
		t.Fatalf("seen pages = %v, want [1 2]", seenPages)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(result.Items))
	}
}

func TestFetchPricesReturnsEmptyResult(t *testing.T) {
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

	client := newTestClient(t, server.URL, 100)
	result, err := client.FetchPrices(context.Background(), PriceQuery{
		Operation: OperationGetETFPriceInfo,
	})
	if err != nil {
		t.Fatalf("fetch prices: %v", err)
	}
	if len(result.Items) != 0 {
		t.Fatalf("items len = %d, want 0", len(result.Items))
	}
}

func TestFetchPricesRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, 100)
	_, err := client.FetchPrices(context.Background(), PriceQuery{
		Operation: OperationGetETFPriceInfo,
	})
	if err == nil {
		t.Fatal("fetch prices error = nil, want remote error")
	}
	for _, want := range []string{"provider=datago", "group=securitiesProductPrice", "operation=getETFPriceInfo", "status=502"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func newTestClient(t *testing.T, baseURL string, numOfRows int) *Client {
	t.Helper()
	client, err := New(Config{
		ServiceKey: "test-key",
		BaseURL:    baseURL,
		NumOfRows:  numOfRows,
	})
	if err != nil {
		t.Fatalf("new datago client: %v", err)
	}
	return client
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
