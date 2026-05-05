package cli

import (
	"bytes"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/financials"
	"github.com/ev3rlit/mwosa/service/daily"
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

func TestWriteCollectResultCSVUsesServiceCSVContract(t *testing.T) {
	var out bytes.Buffer

	err := writeCollectResult(&out, OutputModeCSV, daily.CollectResult{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		ProviderID:   provider.ProviderDataGo,
		Group:        provider.GroupSecuritiesProductPrice,
		Dates:        daily.DateList{"2026-04-24", "2026-04-27"},
		BarsFetched:  10,
		BarsStored:   8,
		RowsAffected: 6,
	})
	if err != nil {
		t.Fatalf("write collect result csv: %v", err)
	}

	want := "market,security_type,provider,group,dates,fetched,stored,rows_affected\nkrx,etf,datago,securitiesProductPrice,2,10,8,6\n"
	if got := out.String(); got != want {
		t.Fatalf("csv output = %q, want %q", got, want)
	}
}

func TestWriteFinancialStatementsTableFlattensStatementLines(t *testing.T) {
	var out bytes.Buffer

	err := writeFinancialStatements(&out, OutputModeTable, []financials.Statement{financialStatementForOutputTest()})
	if err != nil {
		t.Fatalf("write financial statements table: %v", err)
	}

	got := out.String()
	for _, want := range []string{"statement", "year", "account", "income_statement", "2025", "Revenue", "1000", "KRW"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table output missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"provider", "group", "operation", "fakeProvider"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("table output should not include %q:\n%s", unwanted, got)
		}
	}
}

func TestWriteFinancialStatementsCSVFlattensStatementLines(t *testing.T) {
	var out bytes.Buffer

	err := writeFinancialStatements(&out, OutputModeCSV, []financials.Statement{financialStatementForOutputTest()})
	if err != nil {
		t.Fatalf("write financial statements csv: %v", err)
	}

	got := out.String()
	if !strings.HasPrefix(got, "statement,symbol,fiscal_year,fiscal_period,period,account_id,account_name,value,currency,unit\n") {
		t.Fatalf("csv header = %q", got)
	}
	if !strings.Contains(got, "income_statement,005930,2025,FY,annual,ifrs_Revenue,Revenue,1000,KRW,원\n") {
		t.Fatalf("csv output missing financial statement line:\n%s", got)
	}
}

func TestWriteTableRendersAlignedColumns(t *testing.T) {
	var out bytes.Buffer

	err := writeTable(&out, []string{"kind", "name"}, [][]string{{"etf", "한국 ETF"}})
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

func financialStatementForOutputTest() financials.Statement {
	return financials.Statement{
		Provider:     provider.ProviderID("fakeProvider"),
		Group:        provider.GroupID("fakeGroup"),
		Operation:    provider.OperationID("fakeOperation"),
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeStock,
		Statement:    financials.StatementTypeIncomeStatement,
		Symbol:       "005930",
		FiscalYear:   "2025",
		FiscalPeriod: "FY",
		Period:       financials.PeriodTypeAnnual,
		Currency:     "KRW",
		Unit:         "원",
		Lines: []financials.LineItem{
			{AccountID: "ifrs_Revenue", AccountName: "Revenue", Value: "1000"},
		},
	}
}
