package dailybar

import (
	"encoding/json"
	"fmt"
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	coredailybar "github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/storage/ent"
)

func validateBarKey(bar coredailybar.Bar) error {
	if bar.Market == "" || bar.SecurityType == "" || bar.TradingDate == "" || bar.Symbol == "" || bar.Provider == "" || bar.Group == "" {
		return fmt.Errorf("daily bar missing sqlite key provider=%s group=%s market=%s security_type=%s date=%s symbol=%s", bar.Provider, bar.Group, bar.Market, bar.SecurityType, bar.TradingDate, bar.Symbol)
	}
	return nil
}

func encodeExtensions(extensions map[string]string) (string, error) {
	if len(extensions) == 0 {
		return "{}", nil
	}
	bytes, err := json.Marshal(extensions)
	if err != nil {
		return "", fmt.Errorf("encode daily bar extensions: %w", err)
	}
	return string(bytes), nil
}

func decodeExtensions(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return nil, nil
	}
	var extensions map[string]string
	if err := json.Unmarshal([]byte(raw), &extensions); err != nil {
		return nil, fmt.Errorf("decode daily bar extensions: %w", err)
	}
	return extensions, nil
}

func entDailyBarToCanonical(row *ent.DailyBar) (coredailybar.Bar, error) {
	if row == nil {
		return coredailybar.Bar{}, fmt.Errorf("daily bar sqlite row is nil")
	}
	extensions, err := decodeExtensions(row.ExtensionsJSON)
	if err != nil {
		return coredailybar.Bar{}, err
	}
	return coredailybar.Bar{
		Provider:     provider.ProviderID(row.Provider),
		Group:        provider.GroupID(row.ProviderGroup),
		Operation:    provider.OperationID(row.Operation),
		Market:       provider.Market(row.Market),
		SecurityType: provider.SecurityType(row.SecurityType),
		Symbol:       row.Symbol,
		ISIN:         row.Isin,
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
