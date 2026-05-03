package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
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

type RecordSet struct {
	Columns []string
	Rows    [][]string
}

func writeBars(w io.Writer, output OutputMode, bars []dailybar.Bar) error {
	errb := oops.In("cli_output").With("format", output)
	records := dailyBarsRecordSet(bars)

	switch output {
	case "", OutputModeTable:
		return writeTable(w, records)
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
		return writeCSV(w, records)
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func writeCollectResult(w io.Writer, output OutputMode, result daily.CollectResult) error {
	errb := oops.In("cli_output").With("format", output)
	resultErrb := errb.With("provider", result.ProviderID, "group", result.Group)
	records := collectResultRecordSet(result)

	switch output {
	case "", OutputModeTable:
		return writeTable(w, records)
	case OutputModeJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return resultErrb.Wrap(encoder.Encode(result))
	case OutputModeNDJSON:
		return resultErrb.Wrap(json.NewEncoder(w).Encode(result))
	case OutputModeCSV:
		return writeCSV(w, records)
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func writeTable(w io.Writer, records RecordSet) error {
	errb := oops.In("cli_output").With("columns", len(records.Columns), "rows", len(records.Rows))

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
	table.Header(records.Columns)
	if err := table.Bulk(records.Rows); err != nil {
		return errb.Wrapf(err, "write table rows")
	}
	return errb.Wrap(table.Render())
}

func writeCSV(w io.Writer, records RecordSet) error {
	errb := oops.In("cli_output").With("columns", len(records.Columns), "rows", len(records.Rows))
	writer := csv.NewWriter(w)
	if err := writer.Write(records.Columns); err != nil {
		return errb.Wrapf(err, "write csv header")
	}
	for _, row := range records.Rows {
		if err := writer.Write(row); err != nil {
			return errb.Wrapf(err, "write csv row")
		}
	}
	writer.Flush()
	return errb.Wrap(writer.Error())
}

func dailyBarsRecordSet(bars []dailybar.Bar) RecordSet {
	rows := make([][]string, 0, len(bars))
	for _, bar := range bars {
		rows = append(rows, []string{bar.TradingDate, bar.Symbol, bar.Name, bar.Open, bar.High, bar.Low, bar.Close, bar.Change})
	}
	return RecordSet{
		Columns: []string{"date", "symbol", "name", "open", "high", "low", "close", "change"},
		Rows:    rows,
	}
}

func collectResultRecordSet(result daily.CollectResult) RecordSet {
	return RecordSet{
		Columns: []string{"market", "security_type", "provider", "group", "dates", "fetched", "stored", "rows_affected"},
		Rows: [][]string{{
			string(result.Market),
			string(result.SecurityType),
			string(result.ProviderID),
			string(result.Group),
			fmt.Sprint(len(result.Dates)),
			fmt.Sprint(result.BarsFetched),
			fmt.Sprint(result.BarsStored),
			fmt.Sprint(result.RowsAffected),
		}},
	}
}
