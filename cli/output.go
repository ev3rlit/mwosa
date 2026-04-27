package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/samber/oops"
)

func writeBars(w io.Writer, output string, bars []dailybar.Bar) error {
	switch output {
	case "", "table":
		_, _ = fmt.Fprintln(w, "date\tsymbol\tname\tclose\tvolume\tprovider\tgroup\toperation")
		for _, bar := range bars {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", bar.TradingDate, bar.Symbol, bar.Name, bar.Close, bar.Volume, bar.Provider, bar.Group, bar.Operation)
		}
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return oops.In("cli_output").With("format", output, "rows", len(bars)).Wrap(encoder.Encode(bars))
	case "ndjson":
		encoder := json.NewEncoder(w)
		for _, bar := range bars {
			if err := encoder.Encode(bar); err != nil {
				return oops.In("cli_output").With("format", output, "symbol", bar.Symbol).Wrapf(err, "write daily bar ndjson")
			}
		}
		return nil
	case "csv":
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"date", "symbol", "name", "close", "volume", "provider", "group", "operation"}); err != nil {
			return oops.In("cli_output").With("format", output).Wrapf(err, "write daily bar csv header")
		}
		for _, bar := range bars {
			if err := writer.Write([]string{bar.TradingDate, bar.Symbol, bar.Name, bar.Close, bar.Volume, string(bar.Provider), string(bar.Group), string(bar.Operation)}); err != nil {
				return oops.In("cli_output").With("format", output, "symbol", bar.Symbol).Wrapf(err, "write daily bar csv row")
			}
		}
		writer.Flush()
		return oops.In("cli_output").With("format", output).Wrap(writer.Error())
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}

func writeCollectResult(w io.Writer, output string, result daily.CollectResult) error {
	switch output {
	case "", "table":
		_, _ = fmt.Fprintln(w, "market\tsecurity_type\tprovider\tgroup\tdates\tfetched\tstored\trows_affected")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\n", result.Market, result.SecurityType, result.ProviderID, result.Group, len(result.Dates), result.BarsFetched, result.BarsStored, result.RowsAffected)
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return oops.In("cli_output").With("format", output, "provider", result.ProviderID, "group", result.Group).Wrap(encoder.Encode(result))
	case "ndjson":
		return oops.In("cli_output").With("format", output, "provider", result.ProviderID, "group", result.Group).Wrap(json.NewEncoder(w).Encode(result))
	case "csv":
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"market", "security_type", "provider", "group", "dates", "fetched", "stored", "rows_affected"}); err != nil {
			return oops.In("cli_output").With("format", output).Wrapf(err, "write collect csv header")
		}
		if err := writer.Write([]string{
			string(result.Market),
			string(result.SecurityType),
			string(result.ProviderID),
			string(result.Group),
			fmt.Sprint(len(result.Dates)),
			fmt.Sprint(result.BarsFetched),
			fmt.Sprint(result.BarsStored),
			fmt.Sprint(result.RowsAffected),
		}); err != nil {
			return oops.In("cli_output").With("format", output, "provider", result.ProviderID, "group", result.Group).Wrapf(err, "write collect csv row")
		}
		writer.Flush()
		return oops.In("cli_output").With("format", output).Wrap(writer.Error())
	default:
		return oops.In("cli_output").With("format", output).Errorf("unsupported output format: %s", output)
	}
}
