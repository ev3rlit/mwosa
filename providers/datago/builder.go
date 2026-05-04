package datago

import (
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
)

const (
	serviceKeyEnv         = "MWOSA_DATAGO_SERVICE_KEY"
	serviceKeyFallbackEnv = "DATAGO_SERVICE_KEY"
	baseURLEnv            = "MWOSA_DATAGO_BASE_URL"
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
				Required:    true,
				Secret:      true,
				Description: "공공데이터포털 service key",
				Env:         []string{serviceKeyEnv, serviceKeyFallbackEnv},
			},
			{
				Path:        "base_url",
				Flag:        "base-url",
				Description: "override datago API base URL",
				Env:         []string{baseURLEnv},
			},
			{
				Path:        "groups.securitiesProductPrice.enabled",
				Description: "enable datago securitiesProductPrice group",
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
	if !securitiesProductPriceEnabledFromConfig(config) {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   "datago securitiesProductPrice group disabled",
		}
	}
	if serviceKeyFromConfig(config) != "" {
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
	serviceKey := serviceKeyFromConfig(config)
	if serviceKey == "" {
		return nil, oops.In("provider_registry").
			With("provider", provider.ProviderDataGo).
			New("datago provider config requires service key: configure providers.datago.auth.service_key or set MWOSA_DATAGO_SERVICE_KEY or DATAGO_SERVICE_KEY")
	}
	return New(Config{
		ServiceKey: serviceKey,
		BaseURL:    baseURLFromConfig(config),
	})
}

func serviceKeyFromConfig(config provider.Config) string {
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

func baseURLFromConfig(config provider.Config) string {
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

func securitiesProductPriceEnabledFromConfig(config provider.Config) bool {
	enabled, ok := config.Bool("providers", "datago", "groups", string(provider.GroupSecuritiesProductPrice), "enabled")
	return !ok || enabled
}

func requestsDataGo(opts provider.RegisterOptions) bool {
	return opts.ProviderID == provider.ProviderDataGo || opts.PreferProvider == provider.ProviderDataGo
}

func forcedOtherProvider(opts provider.RegisterOptions) bool {
	return opts.ProviderID != "" && opts.ProviderID != provider.ProviderDataGo
}
