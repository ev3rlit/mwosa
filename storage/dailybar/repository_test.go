package dailybar

import (
	"context"
	"path/filepath"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage"
)

func TestDailyBarStoreUpsertIsIdempotent(t *testing.T) {
	database := storage.NewDatabase(filepath.Join(t.TempDir(), "mwosa.db"))
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})
	reader, writer, err := NewRepositories(database)
	if err != nil {
		t.Fatalf("new repositories: %v", err)
	}
	bar := dailybar.Bar{
		Provider:     provider.ProviderDataGo,
		Group:        provider.GroupSecuritiesProductPrice,
		Operation:    provider.OperationGetETFPriceInfo,
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "069500",
		Name:         "KODEX 200",
		TradingDate:  "2024-04-15",
		Close:        "35120",
		Extensions: map[string]string{
			"nav": "35155.1",
		},
	}

	if _, err := writer.UpsertDailyBars(context.Background(), []dailybar.Bar{bar}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	bar.Close = "35130"
	if _, err := writer.UpsertDailyBars(context.Background(), []dailybar.Bar{bar}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	bars, err := reader.QueryDailyBars(context.Background(), daily.Query{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "069500",
		From:         "2024-04-15",
		To:           "2024-04-15",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("bars len = %d, want 1", len(bars))
	}
	if bars[0].Close != "35130" {
		t.Fatalf("close = %q, want updated close 35130", bars[0].Close)
	}
	if bars[0].Extensions["nav"] != "35155.1" {
		t.Fatalf("nav extension = %q, want 35155.1", bars[0].Extensions["nav"])
	}
}

func TestNewRepositoriesRequiresDatabase(t *testing.T) {
	if _, _, err := NewRepositories(nil); err == nil {
		t.Fatal("NewRepositories nil database error is nil")
	}
	if _, err := NewReadRepository(nil); err == nil {
		t.Fatal("NewReadRepository nil database error is nil")
	}
	if _, err := NewWriteRepository(nil); err == nil {
		t.Fatal("NewWriteRepository nil database error is nil")
	}
}
