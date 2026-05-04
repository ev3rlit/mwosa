package cli

import (
	"encoding/json"
	"fmt"
	"io"

	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/samber/oops"
)

type deleteStrategyResult struct {
	Name    string `json:"name" csv:"name"`
	Deleted bool   `json:"deleted" csv:"deleted"`
}

type strategySummary struct {
	Name         string `json:"name" csv:"name"`
	Engine       string `json:"engine" csv:"engine"`
	InputDataset string `json:"input_dataset" csv:"input_dataset"`
	Version      int    `json:"version" csv:"version"`
	QueryHash    string `json:"query_hash" csv:"query_hash"`
}

func writeStrategyDetail(w io.Writer, output OutputMode, detail strategyservice.StrategyDetail) error {
	switch output {
	case "", OutputModeTable:
		return writeTable(w,
			[]string{"name", "engine", "input", "version", "query_hash"},
			[][]string{{
				detail.Strategy.Name,
				string(detail.Strategy.Engine),
				detail.ActiveVersion.InputDataset,
				fmt.Sprint(detail.ActiveVersion.Version),
				detail.ActiveVersion.QueryHash,
			}},
		)
	case OutputModeJSON:
		return writeIndentedJSON(w, detail)
	case OutputModeNDJSON:
		return writeJSONLine(w, detail)
	case OutputModeCSV:
		return writeCSV(w, []strategySummary{strategySummaryFromDetail(detail)})
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func writeStrategyList(w io.Writer, output OutputMode, details []strategyservice.StrategyDetail) error {
	switch output {
	case "", OutputModeTable:
		rows := make([][]string, 0, len(details))
		for _, detail := range details {
			rows = append(rows, []string{
				detail.Strategy.Name,
				string(detail.Strategy.Engine),
				detail.ActiveVersion.InputDataset,
				fmt.Sprint(detail.ActiveVersion.Version),
				detail.ActiveVersion.QueryHash,
			})
		}
		return writeTable(w, []string{"name", "engine", "input", "version", "query_hash"}, rows)
	case OutputModeJSON:
		return writeIndentedJSON(w, details)
	case OutputModeNDJSON:
		return writeNDJSON(w, details)
	case OutputModeCSV:
		rows := make([]strategySummary, 0, len(details))
		for _, detail := range details {
			rows = append(rows, strategySummaryFromDetail(detail))
		}
		return writeCSV(w, rows)
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func strategySummaryFromDetail(detail strategyservice.StrategyDetail) strategySummary {
	return strategySummary{
		Name:         detail.Strategy.Name,
		Engine:       string(detail.Strategy.Engine),
		InputDataset: detail.ActiveVersion.InputDataset,
		Version:      detail.ActiveVersion.Version,
		QueryHash:    detail.ActiveVersion.QueryHash,
	}
}

func writeDeleteStrategyResult(w io.Writer, output OutputMode, result deleteStrategyResult) error {
	switch output {
	case "", OutputModeTable:
		return writeTable(w, []string{"name", "deleted"}, [][]string{{result.Name, fmt.Sprint(result.Deleted)}})
	case OutputModeJSON:
		return writeIndentedJSON(w, result)
	case OutputModeNDJSON:
		return writeJSONLine(w, result)
	case OutputModeCSV:
		return writeCSV(w, []deleteStrategyResult{result})
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func writeScreenRunHistory(w io.Writer, output OutputMode, runs []strategyservice.ScreenRun) error {
	switch output {
	case "", OutputModeTable:
		rows := make([][]string, 0, len(runs))
		for _, run := range runs {
			rows = append(rows, []string{
				run.ID,
				run.Alias,
				string(run.Status),
				run.InputDataset,
				fmt.Sprint(run.ResultCount),
				run.StartedAt.Format(timeLayout),
			})
		}
		return writeTable(w, []string{"id", "alias", "status", "input", "results", "started"}, rows)
	case OutputModeJSON:
		return writeIndentedJSON(w, runs)
	case OutputModeNDJSON:
		return writeNDJSON(w, runs)
	case OutputModeCSV:
		return writeCSV(w, runs)
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func writeScreenRunDetail(w io.Writer, output OutputMode, detail strategyservice.ScreenRunDetail) error {
	switch output {
	case "", OutputModeTable:
		rows := make([][]string, 0, len(detail.Items))
		for _, item := range detail.Items {
			rows = append(rows, []string{fmt.Sprint(item.Ordinal), item.Symbol, string(item.PayloadJSON)})
		}
		if len(rows) == 0 {
			rows = append(rows, []string{"", "", fmt.Sprintf("screen %s %s with %d results", detail.Run.ID, detail.Run.Status, detail.Run.ResultCount)})
		}
		return writeTable(w, []string{"ordinal", "symbol", "payload"}, rows)
	case OutputModeJSON:
		return writeIndentedJSON(w, detail)
	case OutputModeNDJSON:
		return writeNDJSON(w, detail.Items)
	case OutputModeCSV:
		return writeCSV(w, detail.Items)
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func writeScreenResult(w io.Writer, output OutputMode, result strategyservice.ScreenResult) error {
	switch output {
	case "", OutputModeTable:
		rows := make([][]string, 0, len(result.Items))
		for _, item := range result.Items {
			rows = append(rows, []string{fmt.Sprint(item.Ordinal), item.Symbol, string(item.PayloadJSON)})
		}
		if len(rows) == 0 {
			rows = append(rows, []string{"", "", fmt.Sprintf("screen %s with %d results", result.QueryHash, result.ResultCount)})
		}
		return writeTable(w, []string{"ordinal", "symbol", "payload"}, rows)
	case OutputModeJSON:
		return writeIndentedJSON(w, result)
	case OutputModeNDJSON:
		return writeNDJSON(w, result.Items)
	case OutputModeCSV:
		return writeCSV(w, result.Items)
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func writeIndentedJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return oops.In("cli_output").Wrap(encoder.Encode(value))
}

func writeJSONLine(w io.Writer, value any) error {
	return oops.In("cli_output").Wrap(json.NewEncoder(w).Encode(value))
}

func writeNDJSON[T any](w io.Writer, rows []T) error {
	encoder := json.NewEncoder(w)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return oops.In("cli_output").Wrapf(err, "write ndjson row")
		}
	}
	return nil
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
