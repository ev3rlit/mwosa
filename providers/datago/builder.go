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

func (Builder) Decide(opts provider.RegisterOptions, config provider.Config) provider.RegistrationDecision {
	requested := requestsDataGo(opts)
	if forcedOtherProvider(opts) && !requested {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   "another provider is forced",
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

func requestsDataGo(opts provider.RegisterOptions) bool {
	return opts.ProviderID == provider.ProviderDataGo || opts.PreferProvider == provider.ProviderDataGo
}

func forcedOtherProvider(opts provider.RegisterOptions) bool {
	return opts.ProviderID != "" && opts.ProviderID != provider.ProviderDataGo
}
