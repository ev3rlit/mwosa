package cli

import (
	"bytes"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
)

func TestWriteBarsTableShowsPriceFieldsWithoutProviderMetadata(t *testing.T) {
	var out bytes.Buffer

	err := writeBars(&out, OutputModeTable, []dailybar.Bar{dailyBarForOutputTest()})
	if err != nil {
		t.Fatalf("write bars table: %v", err)
	}

	got := out.String()
	for _, want := range []string{"date", "symbol", "name", "open", "high", "low", "close", "change", "2026-04-24", "069500", "KODEX 200", "97000", "99000", "96000", "98000", "-500"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table output missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\t") {
		t.Fatalf("table output should be rendered, not tab-delimited:\n%s", got)
	}
	for _, unwanted := range []string{"┌", "┬", "┐", "│", "├", "┼", "┤", "└", "┴", "┘"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("table output should not include box border %q:\n%s", unwanted, got)
		}
	}
	for _, unwanted := range []string{"provider", "group", "operation", "datago", "securitiesProductPrice", "getETFPriceInfo"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("table output should not include %q:\n%s", unwanted, got)
		}
	}
}

func TestWriteBarsCSVShowsPriceFieldsWithoutProviderMetadata(t *testing.T) {
	var out bytes.Buffer

	err := writeBars(&out, OutputModeCSV, []dailybar.Bar{dailyBarForOutputTest()})
	if err != nil {
		t.Fatalf("write bars csv: %v", err)
	}

	got := out.String()
	if !strings.HasPrefix(got, "date,symbol,name,open,high,low,close,change\n") {
		t.Fatalf("csv header = %q", got)
	}
	for _, unwanted := range []string{"provider", "group", "operation", "datago", "securitiesProductPrice", "getETFPriceInfo"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("csv output should not include %q:\n%s", unwanted, got)
		}
	}
}

func TestWriteTableRendersRecordSet(t *testing.T) {
	var out bytes.Buffer

	err := writeTable(&out, RecordSet{
		Columns: []string{"kind", "name"},
		Rows:    [][]string{{"etf", "한국 ETF"}},
	})
	if err != nil {
		t.Fatalf("write table: %v", err)
	}

	got := out.String()
	for _, want := range []string{"kind", "name", "etf", "한국 ETF"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table output missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"┌", "┬", "┐", "│", "├", "┼", "┤", "└", "┴", "┘"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("table output should not include box border %q:\n%s", unwanted, got)
		}
	}
}

func dailyBarForOutputTest() dailybar.Bar {
	return dailybar.Bar{
		Provider:    provider.ProviderDataGo,
		Group:       provider.GroupSecuritiesProductPrice,
		Operation:   provider.OperationGetETFPriceInfo,
		Symbol:      "069500",
		Name:        "KODEX 200",
		TradingDate: "2026-04-24",
		Open:        "97000",
		High:        "99000",
		Low:         "96000",
		Close:       "98000",
		Change:      "-500",
		Volume:      "16267003",
	}
}
