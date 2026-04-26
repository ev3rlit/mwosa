package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
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
		return encoder.Encode(bars)
	case "ndjson":
		encoder := json.NewEncoder(w)
		for _, bar := range bars {
			if err := encoder.Encode(bar); err != nil {
				return err
			}
		}
		return nil
	case "csv":
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"date", "symbol", "name", "close", "volume", "provider", "group", "operation"}); err != nil {
			return err
		}
		for _, bar := range bars {
			if err := writer.Write([]string{bar.TradingDate, bar.Symbol, bar.Name, bar.Close, bar.Volume, string(bar.Provider), string(bar.Group), string(bar.Operation)}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	default:
		return fmt.Errorf("unsupported output format: %s", output)
	}
}

func writeCollectResult(w io.Writer, output string, result daily.CollectResult) error {
	switch output {
	case "", "table":
		_, _ = fmt.Fprintln(w, "market\tsecurity_type\tprovider\tgroup\tdates\tfetched\tstored\tfiles")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\n", result.Market, result.SecurityType, result.ProviderID, result.Group, len(result.Dates), result.BarsFetched, result.BarsStored, result.FilesWritten)
		return nil
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "ndjson":
		return json.NewEncoder(w).Encode(result)
	case "csv":
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"market", "security_type", "provider", "group", "dates", "fetched", "stored", "files"}); err != nil {
			return err
		}
		if err := writer.Write([]string{
			string(result.Market),
			string(result.SecurityType),
			string(result.ProviderID),
			string(result.Group),
			fmt.Sprint(len(result.Dates)),
			fmt.Sprint(result.BarsFetched),
			fmt.Sprint(result.BarsStored),
			fmt.Sprint(result.FilesWritten),
		}); err != nil {
			return err
		}
		writer.Flush()
		return writer.Error()
	default:
		return fmt.Errorf("unsupported output format: %s", output)
	}
}
