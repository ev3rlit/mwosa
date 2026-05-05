package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/financials"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/jszwec/csvutil"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/samber/oops"
)

type OutputMode string

const (
	DefaultOutputMode OutputMode = OutputModeTable

	OutputModeTable  OutputMode = "table"
	OutputModeJSON   OutputMode = "json"
	OutputModeNDJSON OutputMode = "ndjson"
	OutputModeCSV    OutputMode = "csv"
)

var supportedOutputModes = []OutputMode{
	OutputModeTable,
	OutputModeJSON,
	OutputModeNDJSON,
	OutputModeCSV,
}

func SupportedOutputModeStrings() []string {
	values := make([]string, 0, len(supportedOutputModes))
	for _, mode := range supportedOutputModes {
		values = append(values, string(mode))
	}
	return values
}

func OutputModeHelp() string {
	return "output format: " + strings.Join(SupportedOutputModeStrings(), ", ")
}

func ParseOutputMode(value string) (OutputMode, error) {
	if value == "" {
		return DefaultOutputMode, nil
	}
	for _, mode := range supportedOutputModes {
		if value == string(mode) {
			return mode, nil
		}
	}
	return "", oops.In("cli_output").With("format", value).Errorf("unsupported output format: %s", value)
}

func (m OutputMode) String() string {
	if m == "" {
		return string(DefaultOutputMode)
	}
	return string(m)
}

func (m *OutputMode) Set(value string) error {
	mode, err := ParseOutputMode(value)
	if err != nil {
		return err
	}
	*m = mode
	return nil
}

func (m OutputMode) Type() string {
	return "output"
}

func writeBars(w io.Writer, output OutputMode, bars []dailybar.Bar) error {
	errb := oops.In("cli_output").With("format", output)

	switch output {
	case "", OutputModeTable:
		return writeDailyBarsTable(w, bars)
	case OutputModeJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return errb.With("rows", len(bars)).Wrap(encoder.Encode(bars))
	case OutputModeNDJSON:
		encoder := json.NewEncoder(w)
		for _, bar := range bars {
			if err := encoder.Encode(bar); err != nil {
				return errb.With("symbol", bar.Symbol).Wrapf(err, "write daily bar ndjson")
			}
		}
		return nil
	case OutputModeCSV:
		return writeCSV(w, bars)
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func writeCollectResult(w io.Writer, output OutputMode, result daily.CollectResult) error {
	errb := oops.In("cli_output").With("format", output)
	resultErrb := errb.With("provider", result.ProviderID, "group", result.Group)

	switch output {
	case "", OutputModeTable:
		return writeCollectResultTable(w, result)
	case OutputModeJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return resultErrb.Wrap(encoder.Encode(result))
	case OutputModeNDJSON:
		return resultErrb.Wrap(json.NewEncoder(w).Encode(result))
	case OutputModeCSV:
		return writeCSV(w, []daily.CollectResult{result})
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func writeFinancialStatements(w io.Writer, output OutputMode, statements []financials.Statement) error {
	errb := oops.In("cli_output").With("format", output)

	switch output {
	case "", OutputModeTable:
		return writeFinancialStatementsTable(w, statements)
	case OutputModeJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return errb.With("rows", len(statements)).Wrap(encoder.Encode(statements))
	case OutputModeNDJSON:
		encoder := json.NewEncoder(w)
		for _, statement := range statements {
			if err := encoder.Encode(statement); err != nil {
				return errb.With("symbol", statement.Symbol, "statement", statement.Statement).Wrapf(err, "write financial statement ndjson")
			}
		}
		return nil
	case OutputModeCSV:
		return writeCSV(w, flattenFinancialStatementLines(statements))
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func writeTable(w io.Writer, header []string, rows [][]string) error {
	errb := oops.In("cli_output").With("columns", len(header), "rows", len(rows))
	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Lines:      tw.LinesNone,
				Separators: tw.SeparatorsNone,
			},
			Symbols: tw.NewSymbols(tw.StyleNone),
		})),
		tablewriter.WithHeaderAlignment(tw.AlignLeft),
		tablewriter.WithRowAlignment(tw.AlignLeft),
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithRowAutoFormat(tw.Off),
		tablewriter.WithPadding(tw.Padding{Right: "  ", Overwrite: true}),
	)
	table.Header(header)
	if err := table.Bulk(rows); err != nil {
		return errb.Wrapf(err, "write table rows")
	}
	return errb.Wrap(table.Render())
}

func writeCSV(w io.Writer, rows any) error {
	writer := csv.NewWriter(w)
	encoder := csvutil.NewEncoder(writer)
	if err := encoder.Encode(rows); err != nil {
		return oops.In("cli_output").Wrapf(err, "write csv")
	}
	writer.Flush()
	return oops.In("cli_output").Wrap(writer.Error())
}

func writeDailyBarsTable(w io.Writer, bars []dailybar.Bar) error {
	rows := make([][]string, 0, len(bars))
	for _, bar := range bars {
		rows = append(rows, []string{bar.TradingDate, bar.Symbol, bar.Name, bar.Open, bar.High, bar.Low, bar.Close, bar.Change})
	}
	return writeTable(w, []string{"date", "symbol", "name", "open", "high", "low", "close", "change"}, rows)
}

func writeCollectResultTable(w io.Writer, result daily.CollectResult) error {
	return writeTable(w,
		[]string{"market", "security_type", "provider", "group", "dates", "fetched", "stored", "rows_affected"},
		[][]string{{
			string(result.Market),
			string(result.SecurityType),
			string(result.ProviderID),
			string(result.Group),
			fmt.Sprint(len(result.Dates)),
			fmt.Sprint(result.BarsFetched),
			fmt.Sprint(result.BarsStored),
			fmt.Sprint(result.RowsAffected),
		}},
	)
}

func writeFinancialStatementsTable(w io.Writer, statements []financials.Statement) error {
	rows := make([][]string, 0)
	for _, statement := range statements {
		for _, line := range statement.Lines {
			rows = append(rows, []string{
				string(statement.Statement),
				statement.FiscalYear,
				string(statement.Period),
				line.AccountName,
				line.Value,
				firstNonEmpty(line.Currency, statement.Currency),
				firstNonEmpty(line.Unit, statement.Unit),
			})
		}
	}
	return writeTable(w, []string{"statement", "year", "period", "account", "value", "currency", "unit"}, rows)
}

type financialStatementLineRow struct {
	Statement    financials.StatementType `csv:"statement"`
	Symbol       string                   `csv:"symbol"`
	FiscalYear   string                   `csv:"fiscal_year"`
	FiscalPeriod string                   `csv:"fiscal_period"`
	Period       financials.PeriodType    `csv:"period"`
	AccountID    string                   `csv:"account_id"`
	AccountName  string                   `csv:"account_name"`
	Value        string                   `csv:"value"`
	Currency     string                   `csv:"currency"`
	Unit         string                   `csv:"unit"`
}

func flattenFinancialStatementLines(statements []financials.Statement) []financialStatementLineRow {
	rows := make([]financialStatementLineRow, 0)
	for _, statement := range statements {
		for _, line := range statement.Lines {
			rows = append(rows, financialStatementLineRow{
				Statement:    statement.Statement,
				Symbol:       statement.Symbol,
				FiscalYear:   statement.FiscalYear,
				FiscalPeriod: statement.FiscalPeriod,
				Period:       statement.Period,
				AccountID:    line.AccountID,
				AccountName:  line.AccountName,
				Value:        line.Value,
				Currency:     firstNonEmpty(line.Currency, statement.Currency),
				Unit:         firstNonEmpty(line.Unit, statement.Unit),
			})
		}
	}
	return rows
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
