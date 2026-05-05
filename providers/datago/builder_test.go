package datago_test

import (
	"context"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/datago"
)

func TestRegistrySkipsDataGoWhenConfigMissingAndProviderUnspecified(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{}, provider.Config{}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfigured error = %v", err)
	}

	_, err = dailybar.NewRouter(provider.NewRouter(registry)).RouteDailyBars(context.Background(), dailybar.RouteInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	})
	if !provider.IsNoProvider(err) {
		t.Fatalf("RouteDailyBars error = %v, want ErrNoProvider", err)
	}
}

func TestRegistryErrorsWhenDataGoProviderRequestedWithoutKey(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{
		ProviderID: provider.ProviderDataGo,
	}, provider.Config{}, datago.NewBuilder())
	assertDataGoKeyError(t, err)
}

func TestRegistryRegistersDataGoWhenConfigObjectContainsServiceKey(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{}, provider.Config{
		"providers": map[string]any{
			"datago": map[string]any{
				"auth": map[string]any{
					"service_key": "test-key",
				},
			},
		},
	}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfigured error = %v", err)
	}

	assertDataGoDailyBarRoute(t, registry, dailybar.RouteInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	}, provider.GroupSecuritiesProductPrice)
}

func TestRegistryRegistersOnlyStockPriceGroupWhenStockKeyPresent(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{}, provider.Config{
		"providers": map[string]any{
			"datago": map[string]any{
				"groups": map[string]any{
					"stockPrice": map[string]any{
						"auth": map[string]any{
							"service_key": "stock-key",
						},
					},
				},
			},
		},
	}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfigured error = %v", err)
	}

	assertDataGoDailyBarRoute(t, registry, dailybar.RouteInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeStock,
	}, provider.GroupStockPrice)

	_, err = dailybar.NewRouter(provider.NewRouter(registry)).RouteDailyBars(context.Background(), dailybar.RouteInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	})
	if !provider.IsNoProvider(err) {
		t.Fatalf("RouteDailyBars ETF error = %v, want ErrNoProvider", err)
	}
}

func TestRegistrySkipsDataGoWhenConfigDisablesProvider(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{}, provider.Config{
		"providers": map[string]any{
			"datago": map[string]any{
				"enabled": false,
				"auth": map[string]any{
					"service_key": "test-key",
				},
			},
		},
	}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfigured error = %v", err)
	}

	_, err = dailybar.NewRouter(provider.NewRouter(registry)).RouteDailyBars(context.Background(), dailybar.RouteInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	})
	if !provider.IsNoProvider(err) {
		t.Fatalf("RouteDailyBars error = %v, want ErrNoProvider", err)
	}
}

func TestRegistrySkipsDataGoWhenSecuritiesProductPriceGroupDisabled(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{}, provider.Config{
		"providers": map[string]any{
			"datago": map[string]any{
				"auth": map[string]any{
					"service_key": "test-key",
				},
				"groups": map[string]any{
					"securitiesProductPrice": map[string]any{
						"enabled": false,
					},
				},
			},
		},
	}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfigured error = %v", err)
	}

	_, err = dailybar.NewRouter(provider.NewRouter(registry)).RouteDailyBars(context.Background(), dailybar.RouteInput{
		ProviderID:   provider.ProviderDataGo,
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	})
	if !provider.IsNoProvider(err) {
		t.Fatalf("RouteDailyBars error = %v, want ErrNoProvider", err)
	}
}

func TestRegistryRegistersDataGoWhenProviderRequestedAndEnvConfigPresent(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{
		ProviderID: provider.ProviderDataGo,
	}, provider.Config{
		"env": map[string]any{
			"MWOSA_DATAGO_SERVICE_KEY": "test-key",
		},
	}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfigured error = %v", err)
	}

	assertDataGoDailyBarRoute(t, registry, dailybar.RouteInput{
		ProviderID:   provider.ProviderDataGo,
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	}, provider.GroupSecuritiesProductPrice)
}

func TestRegistryRegisterConfiguredFromEnvUsesDataGoFallbackServiceKey(t *testing.T) {
	t.Setenv("MWOSA_DATAGO_SERVICE_KEY", "")
	t.Setenv("DATAGO_SERVICE_KEY", "fallback-key")
	t.Setenv("MWOSA_DATAGO_BASE_URL", "")

	registry := provider.NewRegistry()
	err := registry.RegisterConfiguredFromEnv(provider.RegisterOptions{}, datago.NewBuilder())
	if err != nil {
		t.Fatalf("RegisterConfiguredFromEnv error = %v", err)
	}

	assertDataGoDailyBarRoute(t, registry, dailybar.RouteInput{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
	}, provider.GroupSecuritiesProductPrice)
}

func TestRegistryErrorsWhenDataGoPreferredWithoutKey(t *testing.T) {
	registry := provider.NewRegistry()
	err := registry.RegisterConfigured(provider.RegisterOptions{
		PreferProvider: provider.ProviderDataGo,
	}, provider.Config{}, datago.NewBuilder())
	assertDataGoKeyError(t, err)
}

func assertDataGoDailyBarRoute(t *testing.T, registry *provider.Registry, input dailybar.RouteInput, expectedGroup provider.GroupID) {
	t.Helper()

	fetcher, err := dailybar.NewRouter(provider.NewRouter(registry)).RouteDailyBars(context.Background(), input)
	if err != nil {
		t.Fatalf("RouteDailyBars error = %v", err)
	}
	profile := fetcher.DailyBarProfile()
	if profile.Group != expectedGroup {
		t.Fatalf("profile group = %q, want %q", profile.Group, expectedGroup)
	}
}

func assertDataGoKeyError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("RegisterConfigured error = nil, want datago service key error")
	}
	for _, want := range []string{"datago", "providers.datago.groups.securitiesProductPrice.auth.service_key", "providers.datago.groups.stockPrice.auth.service_key"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}
