package krxlisted

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetItemInfoBuildsQueryAndParsesTypedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getItemInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		assertCommonQuery(t, r, "1", "100")
		assertQuery(t, r, "likeSrtnCd", "005930")
		fmt.Fprint(w, `{
			"response": {
				"header": {"resultCode": "00", "resultMsg": "OK"},
				"body": {
					"numOfRows": 100,
					"pageNo": 1,
					"totalCount": 1,
					"items": {
						"item": {
							"basDt": "20260430",
							"srtnCd": "005930",
							"isinCd": "KR7005930003",
							"mrktCtg": "KOSPI",
							"itmsNm": "삼성전자",
							"crno": 1301110006246,
							"corpNm": "삼성전자주식회사"
						}
					}
				}
			}
		}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetItemInfo(context.Background(), Query{
		LikeSrtnCd: "005930",
	})
	if err != nil {
		t.Fatalf("get item info: %v", err)
	}
	if result.TotalCount != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %+v, want one item", result)
	}
	item := result.Items[0]
	if item.SrtnCd != "005930" || item.Crno != "1301110006246" || item.Fields()["corpNm"] != "삼성전자주식회사" {
		t.Fatalf("unexpected parsed listed item: %+v", item)
	}
}

func TestGetAllItemInfoCollectsAllPages(t *testing.T) {
	seenPages := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getItemInfo" {
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
						{"basDt": "20260430", "srtnCd": "005930", "itmsNm": "삼성전자", "crno": "1301110006246"}
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
						{"basDt": "20260430", "srtnCd": "005931", "itmsNm": "삼성전자우", "crno": "1301110006246"}
					]}
				}
			}`)
		default:
			t.Fatalf("unexpected pageNo: %s", pageNo)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	result, err := client.GetAllItemInfo(context.Background(), Query{
		LikeItmsNm: "삼성전자",
	})
	if err != nil {
		t.Fatalf("get all item info: %v", err)
	}
	if strings.Join(seenPages, ",") != "1,2" {
		t.Fatalf("seen pages = %v, want [1 2]", seenPages)
	}
	if len(result.Items) != 2 || result.Items[1].SrtnCd != "005931" {
		t.Fatalf("items = %+v, want page-order listed rows", result.Items)
	}
}

func TestGetItemInfoRemoteErrorIncludesProviderContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.GetItemInfo(context.Background(), Query{})
	if err == nil {
		t.Fatal("get item info error = nil, want remote error")
	}
	for _, want := range []string{"provider=datago", "group=krxListedInfo", "operation=getItemInfo", "status=502"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestGetItemInfoRetriesTransientStatus(t *testing.T) {
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
				"items": {"item": {"srtnCd": "005930", "crno": "1301110006246"}}
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
		t.Fatalf("new datago krx listed client: %v", err)
	}
	result, err := client.GetItemInfo(context.Background(), Query{})
	if err != nil {
		t.Fatalf("get item info: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(result.Items) != 1 || result.Items[0].Crno != "1301110006246" {
		t.Fatalf("items = %+v, want retried listed row", result.Items)
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
		t.Fatalf("new datago krx listed client: %v", err)
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
