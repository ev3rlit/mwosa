package core_test

import (
	"context"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/spec"
)

type registryTestProvider struct {
	provider.Identity

	DailyBars dailybar.Fetcher
}

func TestRegisterProviderCollectsPublicRoleFields(t *testing.T) {
	registry := provider.NewRegistry()
	p := &registryTestProvider{
		Identity: provider.Identity{
			ID:          provider.ProviderID("test"),
			DisplayName: "test",
		},
		DailyBars: spec.PreviousBusinessDayDailyBar(func(context.Context, dailybar.FetchInput) (dailybar.FetchResult, error) {
			return dailybar.FetchResult{}, nil
		}).
			Markets(provider.MarketKRX).
			SecurityTypes(provider.SecurityTypeETF).
			Group(provider.GroupID("testGroup")).
			Operations(provider.OperationID("testOperation")).
			RequiresAuth(provider.CredentialScopeDataGo).
			RangeQuery(dailybar.RangeQuerySupported).
			MustBuild(),
	}

	if err := registry.RegisterProvider(p); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	entries := registry.Entries(provider.RoleDailyBar)
	if len(entries) != 1 {
		t.Fatalf("dailybar entries len = %d, want 1", len(entries))
	}
	if entries[0].Provider.ID != provider.ProviderID("test") {
		t.Fatalf("provider id = %s, want test", entries[0].Provider.ID)
	}
	if entries[0].Profile.Compatibility.DataLatency != provider.DataLatencyPreviousBusinessDay {
		t.Fatalf("data latency = %s, want previous_business_day", entries[0].Profile.Compatibility.DataLatency)
	}
	if _, ok := entries[0].Impl.(dailybar.Fetcher); !ok {
		t.Fatalf("impl type = %T, want dailybar.Fetcher", entries[0].Impl)
	}
}

func TestRegisterProviderRejectsNilPublicRoleField(t *testing.T) {
	registry := provider.NewRegistry()
	p := &registryTestProvider{
		Identity: provider.Identity{
			ID:          provider.ProviderID("test"),
			DisplayName: "test",
		},
	}

	err := registry.RegisterProvider(p)
	if err == nil {
		t.Fatal("register provider error = nil, want nil role field error")
	}
	if !strings.Contains(err.Error(), "role field is nil") {
		t.Fatalf("error = %q, want role field context", err.Error())
	}
}
