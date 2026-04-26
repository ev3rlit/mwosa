package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestEnsureDailyFetchesBatchAndGetReadsStoredData(t *testing.T) {
	requests := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RawQuery)
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("likeSrtnCd"); got != "" {
			t.Fatalf("ensure should collect the date batch, got likeSrtnCd=%q", got)
		}
		if got := r.URL.Query().Get("basDt"); got != "20240415" {
			t.Fatalf("basDt = %q, want 20240415", got)
		}
		fmt.Fprint(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 2,
				"items": {"item": [
					{"basDt": "20240415", "srtnCd": "069500", "itmsNm": "KODEX 200", "clpr": "35120", "trqu": "10"},
					{"basDt": "20240415", "srtnCd": "123456", "itmsNm": "OTHER ETF", "clpr": "1000", "trqu": "20"}
				]}
			}
		}`)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	setDataGoEnv(t, server.URL)

	var ensureOut bytes.Buffer
	ensureCmd := NewRootCommand(BuildInfo{})
	ensureCmd.SetOut(&ensureOut)
	ensureCmd.SetErr(&ensureOut)
	if err := executeForTest(context.Background(), ensureCmd,
		"--data-dir", dataDir,
		"--output", "json",
		"ensure", "daily", "069500",
		"--as-of", "20240415",
	); err != nil {
		t.Fatalf("ensure daily: %v\n%s", err, ensureOut.String())
	}
	if len(requests) != 1 {
		t.Fatalf("requests len = %d, want 1", len(requests))
	}
	if !strings.Contains(ensureOut.String(), `"symbol": "069500"`) {
		t.Fatalf("ensure output should include requested symbol:\n%s", ensureOut.String())
	}

	var getOut bytes.Buffer
	getCmd := NewRootCommand(BuildInfo{})
	getCmd.SetOut(&getOut)
	getCmd.SetErr(&getOut)
	if err := executeForTest(context.Background(), getCmd,
		"--data-dir", dataDir,
		"--output", "json",
		"get", "daily", "069500",
		"--as-of", "2024-04-15",
	); err != nil {
		t.Fatalf("get daily: %v\n%s", err, getOut.String())
	}
	if !strings.Contains(getOut.String(), `"closing_price": "35120"`) {
		t.Fatalf("get output should include stored close:\n%s", getOut.String())
	}
}

func TestGetDailyMissingDataReturnsEnsureHint(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(BuildInfo{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := executeForTest(context.Background(), cmd,
		"--data-dir", t.TempDir(),
		"get", "daily", "069500",
		"--from", "20240101",
		"--to", "20240102",
	)
	if err == nil {
		t.Fatal("get daily error = nil, want missing data error")
	}
	for _, want := range []string{"daily data not found", "symbol=069500", "mwosa ensure daily 069500"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestBackfillDailyCollectsEachDate(t *testing.T) {
	seenDates := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getETFPriceInfo" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		basDt := r.URL.Query().Get("basDt")
		seenDates = append(seenDates, basDt)
		fmt.Fprintf(w, `{
			"header": {"resultCode": "00", "resultMsg": "OK"},
			"body": {
				"numOfRows": 100,
				"pageNo": 1,
				"totalCount": 1,
				"items": {"item": {"basDt": %q, "srtnCd": "069500", "itmsNm": "KODEX 200", "clpr": "35120"}}
			}
		}`, basDt)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	setDataGoEnv(t, server.URL)

	var out bytes.Buffer
	cmd := NewRootCommand(BuildInfo{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := executeForTest(context.Background(), cmd,
		"--data-dir", dataDir,
		"--output", "json",
		"backfill", "daily",
		"--from", "20240415",
		"--to", "20240416",
	); err != nil {
		t.Fatalf("backfill daily: %v\n%s", err, out.String())
	}
	if strings.Join(seenDates, ",") != "20240415,20240416" {
		t.Fatalf("seen dates = %v, want 20240415 and 20240416", seenDates)
	}
	if !strings.Contains(out.String(), `"bars_fetched": 2`) {
		t.Fatalf("backfill output should summarize fetched bars:\n%s", out.String())
	}
}

func setDataGoEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("MWOSA_DATAGO_SERVICE_KEY", "test-key")
	t.Setenv("MWOSA_DATAGO_BASE_URL", baseURL)
	t.Setenv("MWOSA_DATAGO_NUM_OF_ROWS", "100")
}

func executeForTest(ctx context.Context, cmd *cobra.Command, args ...string) error {
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}
