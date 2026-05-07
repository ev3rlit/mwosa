package datago

import (
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
)

const (
	serviceKeyEnv              = "MWOSA_DATAGO_SERVICE_KEY"
	serviceKeyFallbackEnv      = "DATAGO_SERVICE_KEY"
	baseURLEnv                 = "MWOSA_DATAGO_BASE_URL"
	etpServiceKeyEnv           = "MWOSA_DATAGO_ETP_SERVICE_KEY"
	etpServiceKeyFallbackEnv   = "DATAGO_ETP_SERVICE_KEY"
	etpBaseURLEnv              = "MWOSA_DATAGO_ETP_BASE_URL"
	stockServiceKeyEnv         = "MWOSA_DATAGO_STOCK_PRICE_SERVICE_KEY"
	stockServiceKeyFallbackEnv = "DATAGO_STOCK_PRICE_SERVICE_KEY"
	stockBaseURLEnv            = "MWOSA_DATAGO_STOCK_PRICE_BASE_URL"
)

type Builder struct{}

var _ provider.ProviderBuilder = Builder{}

func NewBuilder() Builder {
	return Builder{}
}

func (Builder) ID() provider.ProviderID {
	return provider.ProviderDataGo
}

func (Builder) DefaultConfig() provider.Config {
	return provider.Config{
		"id":       string(provider.ProviderDataGo),
		"enabled":  true,
		"base_url": "",
		"auth": map[string]any{
			"service_key": "",
		},
		"groups": map[string]any{
			string(provider.GroupSecuritiesProductPrice): map[string]any{
				"enabled": true,
				"auth": map[string]any{
					"service_key": "",
				},
				"base_url": "",
			},
			string(provider.GroupStockPrice): map[string]any{
				"enabled": true,
				"auth": map[string]any{
					"service_key": "",
				},
				"base_url": "",
			},
		},
	}
}

func (Builder) ConfigSpec() provider.ConfigSpec {
	return provider.ConfigSpec{
		ProviderID: provider.ProviderDataGo,
		Fields: []provider.ConfigField{
			{
				Path:        "auth.service_key",
				Flag:        "service-key",
				Required:    false,
				Secret:      true,
				Description: "legacy 공공데이터포털 service key for securitiesProductPrice",
				Env:         []string{serviceKeyEnv, serviceKeyFallbackEnv},
			},
			{
				Path:        "groups.securitiesProductPrice.auth.service_key",
				Flag:        "etp-service-key",
				Required:    false,
				Secret:      true,
				Description: "공공데이터포털 securitiesProductPrice service key",
				Env:         []string{etpServiceKeyEnv, etpServiceKeyFallbackEnv},
			},
			{
				Path:        "groups.stockPrice.auth.service_key",
				Flag:        "stock-price-service-key",
				Required:    false,
				Secret:      true,
				Description: "공공데이터포털 stockPrice service key",
				Env:         []string{stockServiceKeyEnv, stockServiceKeyFallbackEnv},
			},
			{
				Path:        "base_url",
				Flag:        "base-url",
				Description: "legacy override datago securitiesProductPrice API base URL",
				Env:         []string{baseURLEnv},
			},
			{
				Path:        "groups.securitiesProductPrice.base_url",
				Flag:        "etp-base-url",
				Description: "override datago securitiesProductPrice API base URL",
				Env:         []string{etpBaseURLEnv},
			},
			{
				Path:        "groups.stockPrice.base_url",
				Flag:        "stock-price-base-url",
				Description: "override datago stockPrice API base URL",
				Env:         []string{stockBaseURLEnv},
			},
			{
				Path:        "groups.securitiesProductPrice.enabled",
				Description: "enable datago securitiesProductPrice group",
			},
			{
				Path:        "groups.stockPrice.enabled",
				Description: "enable datago stockPrice group",
			},
		},
	}
}

func (Builder) Decide(opts provider.RegisterOptions, config provider.Config) provider.RegistrationDecision {
	if !enabledFromConfig(config) {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   "datago disabled",
		}
	}
	requested := requestsDataGo(opts)
	if forcedOtherProvider(opts) && !requested {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   "another provider is forced",
		}
	}
	if !anyGroupEnabledFromConfig(config) {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   "datago groups disabled",
		}
	}
	if hasAnyGroupServiceKey(config) {
		return provider.RegistrationDecision{
			Register: true,
			Reason:   "datago config is present",
		}
	}
	if requested {
		return provider.RegistrationDecision{
			Register: true,
			Reason:   "datago requested",
		}
	}
	return provider.RegistrationDecision{
		Register: false,
		Reason:   "datago config missing",
	}
}

func (Builder) Build(config provider.Config) (provider.IdentityProvider, error) {
	etpServiceKey := etpServiceKeyFromConfig(config)
	stockServiceKey := stockServiceKeyFromConfig(config)
	if etpServiceKey == "" && stockServiceKey == "" {
		return nil, oops.In("provider_registry").
			With("provider", provider.ProviderDataGo).
			New("datago provider config requires at least one group service key: configure providers.datago.groups.securitiesProductPrice.auth.service_key or providers.datago.groups.stockPrice.auth.service_key")
	}
	return New(Config{
		ServiceKey: legacyServiceKeyFromConfig(config),
		BaseURL:    legacyBaseURLFromConfig(config),
		SecuritiesProductPrice: GroupConfig{
			ServiceKey: etpServiceKey,
			BaseURL:    etpBaseURLFromConfig(config),
		},
		StockPrice: GroupConfig{
			ServiceKey: stockServiceKey,
			BaseURL:    stockBaseURLFromConfig(config),
		},
	})
}

func hasAnyGroupServiceKey(config provider.Config) bool {
	return etpServiceKeyFromConfig(config) != "" || stockServiceKeyFromConfig(config) != ""
}

func etpServiceKeyFromConfig(config provider.Config) string {
	if !groupEnabledFromConfig(config, provider.GroupSecuritiesProductPrice) {
		return ""
	}
	serviceKey := strings.TrimSpace(config.String("providers", "datago", "groups", string(provider.GroupSecuritiesProductPrice), "auth", "service_key"))
	if serviceKey != "" {
		return serviceKey
	}
	serviceKey = strings.TrimSpace(config.Env(etpServiceKeyEnv))
	if serviceKey != "" {
		return serviceKey
	}
	serviceKey = strings.TrimSpace(config.Env(etpServiceKeyFallbackEnv))
	if serviceKey != "" {
		return serviceKey
	}
	return legacyServiceKeyFromConfig(config)
}

func stockServiceKeyFromConfig(config provider.Config) string {
	if !groupEnabledFromConfig(config, provider.GroupStockPrice) {
		return ""
	}
	serviceKey := strings.TrimSpace(config.String("providers", "datago", "groups", string(provider.GroupStockPrice), "auth", "service_key"))
	if serviceKey != "" {
		return serviceKey
	}
	serviceKey = strings.TrimSpace(config.Env(stockServiceKeyEnv))
	if serviceKey != "" {
		return serviceKey
	}
	return strings.TrimSpace(config.Env(stockServiceKeyFallbackEnv))
}

func legacyServiceKeyFromConfig(config provider.Config) string {
	serviceKey := strings.TrimSpace(config.String("providers", "datago", "auth", "service_key"))
	if serviceKey != "" {
		return serviceKey
	}
	serviceKey = strings.TrimSpace(config.Env(serviceKeyEnv))
	if serviceKey != "" {
		return serviceKey
	}
	return strings.TrimSpace(config.Env(serviceKeyFallbackEnv))
}

func etpBaseURLFromConfig(config provider.Config) string {
	baseURL := strings.TrimSpace(config.String("providers", "datago", "groups", string(provider.GroupSecuritiesProductPrice), "base_url"))
	if baseURL != "" {
		return baseURL
	}
	baseURL = strings.TrimSpace(config.Env(etpBaseURLEnv))
	if baseURL != "" {
		return baseURL
	}
	return legacyBaseURLFromConfig(config)
}

func stockBaseURLFromConfig(config provider.Config) string {
	baseURL := strings.TrimSpace(config.String("providers", "datago", "groups", string(provider.GroupStockPrice), "base_url"))
	if baseURL != "" {
		return baseURL
	}
	return strings.TrimSpace(config.Env(stockBaseURLEnv))
}

func legacyBaseURLFromConfig(config provider.Config) string {
	baseURL := strings.TrimSpace(config.String("providers", "datago", "base_url"))
	if baseURL != "" {
		return baseURL
	}
	return strings.TrimSpace(config.Env(baseURLEnv))
}

func enabledFromConfig(config provider.Config) bool {
	enabled, ok := config.Bool("providers", "datago", "enabled")
	return !ok || enabled
}

func groupEnabledFromConfig(config provider.Config, group provider.GroupID) bool {
	enabled, ok := config.Bool("providers", "datago", "groups", string(group), "enabled")
	return !ok || enabled
}

func anyGroupEnabledFromConfig(config provider.Config) bool {
	return groupEnabledFromConfig(config, provider.GroupSecuritiesProductPrice) ||
		groupEnabledFromConfig(config, provider.GroupStockPrice)
}

func requestsDataGo(opts provider.RegisterOptions) bool {
	return opts.ProviderID == provider.ProviderDataGo || opts.PreferProvider == provider.ProviderDataGo
}

func forcedOtherProvider(opts provider.RegisterOptions) bool {
	return opts.ProviderID != "" && opts.ProviderID != provider.ProviderDataGo
}
