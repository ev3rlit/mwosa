package dailybar

import (
	"encoding/json"
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/storage"
	"github.com/samber/oops"
)

func validateBarKey(bar coredailybar.Bar) error {
	if bar.Market == "" || bar.SecurityType == "" || bar.TradingDate == "" || bar.Symbol == "" || bar.Provider == "" || bar.Group == "" {
		return oops.In("dailybar_repository").With(
			"provider", bar.Provider,
			"group", bar.Group,
			"market", bar.Market,
			"security_type", bar.SecurityType,
			"date", bar.TradingDate,
			"symbol", bar.Symbol,
		).New("daily bar missing sqlite key")
	}
	return nil
}

func encodeExtensions(extensions map[string]string) (string, error) {
	if len(extensions) == 0 {
		return "{}", nil
	}
	bytes, err := json.Marshal(extensions)
	if err != nil {
		return "", oops.In("dailybar_repository").Wrapf(err, "encode daily bar extensions")
	}
	return string(bytes), nil
}

func decodeExtensions(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return nil, nil
	}
	var extensions map[string]string
	if err := json.Unmarshal([]byte(raw), &extensions); err != nil {
		return nil, oops.In("dailybar_repository").With("raw", raw).Wrapf(err, "decode daily bar extensions")
	}
	return extensions, nil
}

func dailyBarRowToCanonical(row *storage.DailyBarRow) (coredailybar.Bar, error) {
	if row == nil {
		return coredailybar.Bar{}, oops.In("dailybar_repository").New("daily bar sqlite row is nil")
	}
	extensions, err := decodeExtensions(row.ExtensionsJSON)
	if err != nil {
		return coredailybar.Bar{}, oops.In("dailybar_repository").With(
			"provider", row.Provider,
			"group", row.ProviderGroup,
			"market", row.Market,
			"security_type", row.SecurityType,
			"date", row.TradingDate,
			"symbol", row.Symbol,
		).Wrap(err)
	}
	return coredailybar.Bar{
		Provider:     provider.ProviderID(row.Provider),
		Group:        provider.GroupID(row.ProviderGroup),
		Operation:    provider.OperationID(row.Operation),
		Market:       provider.Market(row.Market),
		SecurityType: provider.SecurityType(row.SecurityType),
		Symbol:       row.Symbol,
		ISIN:         row.ISIN,
		Name:         row.Name,
		TradingDate:  row.TradingDate,
		Currency:     row.Currency,
		Open:         row.OpeningPrice,
		High:         row.HighestPrice,
		Low:          row.LowestPrice,
		Close:        row.ClosingPrice,
		Change:       row.PriceChangeFromPreviousClose,
		ChangeRate:   row.PriceChangeRateFromPreviousClose,
		Volume:       row.TradedVolume,
		TradedValue:  row.TradedAmount,
		MarketCap:    row.MarketCapitalization,
		Extensions:   extensions,
	}, nil
}
