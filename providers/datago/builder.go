package datago

import (
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
)

const (
	serviceKeyEnv                    = "MWOSA_DATAGO_SERVICE_KEY"
	serviceKeyFallbackEnv            = "DATAGO_SERVICE_KEY"
	baseURLEnv                       = "MWOSA_DATAGO_BASE_URL"
	corporateFinanceServiceKeyEnv    = "MWOSA_DATAGO_CORPFIN_SERVICE_KEY"
	corporateFinanceBaseURLEnv       = "MWOSA_DATAGO_CORPFIN_BASE_URL"
	krxListedInfoServiceKeyEnv       = "MWOSA_DATAGO_KRX_LISTED_SERVICE_KEY"
	krxListedInfoBaseURLEnv          = "MWOSA_DATAGO_KRX_LISTED_BASE_URL"
	corporateFinanceDependencyConfig = "dependencies.krxListedInfo"
)

type Builder struct{}
type CorporateFinanceBuilder struct{}

var _ provider.ProviderBuilder = Builder{}
var _ provider.ProviderBuilder = CorporateFinanceBuilder{}

func NewBuilder() Builder {
	return Builder{}
}

func NewCorporateFinanceBuilder() CorporateFinanceBuilder {
	return CorporateFinanceBuilder{}
}

func (Builder) ID() provider.ProviderID {
	return provider.ProviderDataGo
}

func (CorporateFinanceBuilder) ID() provider.ProviderID {
	return provider.ProviderDataGoCorporateFinance
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

func (CorporateFinanceBuilder) DefaultConfig() provider.Config {
	return provider.Config{
		"id":       string(provider.ProviderDataGoCorporateFinance),
		"enabled":  true,
		"base_url": "",
		"auth": map[string]any{
			"service_key": "",
		},
		"groups": map[string]any{
			string(provider.GroupCorporateFinance): map[string]any{
				"enabled": true,
			},
		},
		"dependencies": map[string]any{
			string(provider.GroupKRXListedInfo): map[string]any{
				"base_url": "",
				"auth": map[string]any{
					"service_key": "",
				},
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
				Description: "공공데이터포털 securitiesProductPrice service key",
				Env:         []string{serviceKeyEnv, serviceKeyFallbackEnv},
			},
			{
				Path:        "base_url",
				Flag:        "base-url",
				Description: "override datago securitiesProductPrice API base URL",
				Env:         []string{baseURLEnv},
			},
			{
				Path:        "groups.securitiesProductPrice.enabled",
				Description: "enable datago securitiesProductPrice group",
			},
		},
	}
}

func (CorporateFinanceBuilder) ConfigSpec() provider.ConfigSpec {
	return provider.ConfigSpec{
		ProviderID: provider.ProviderDataGoCorporateFinance,
		Fields: []provider.ConfigField{
			{
				Path:        "auth.service_key",
				Flag:        "service-key",
				Required:    true,
				Secret:      true,
				Description: "공공데이터포털 corporateFinance service key",
				Env:         []string{corporateFinanceServiceKeyEnv, serviceKeyEnv, serviceKeyFallbackEnv},
			},
			{
				Path:        "base_url",
				Flag:        "corpfin-base-url",
				Description: "override datago corporateFinance API base URL",
				Env:         []string{corporateFinanceBaseURLEnv},
			},
			{
				Path:        corporateFinanceDependencyConfig + ".auth.service_key",
				Flag:        "krx-listed-service-key",
				Required:    true,
				Secret:      true,
				Description: "공공데이터포털 krxListedInfo service key used for company-name and KRX-code resolution",
				Env:         []string{krxListedInfoServiceKeyEnv, serviceKeyEnv, serviceKeyFallbackEnv},
			},
			{
				Path:        corporateFinanceDependencyConfig + ".base_url",
				Flag:        "krx-listed-base-url",
				Description: "override datago krxListedInfo API base URL",
				Env:         []string{krxListedInfoBaseURLEnv},
			},
			{
				Path:        "groups.corporateFinance.enabled",
				Description: "enable datago-corpfin corporateFinance group",
			},
		},
	}
}

func (Builder) Decide(opts provider.RegisterOptions, config provider.Config) provider.RegistrationDecision {
	return decideProvider(decideProviderInput{
		ID:           provider.ProviderDataGo,
		Label:        "datago",
		Options:      opts,
		Config:       config,
		ServiceKey:   serviceKeyFromConfig(config),
		GroupEnabled: securitiesProductPriceEnabledFromConfig(config),
		GroupReason:  "datago securitiesProductPrice group disabled",
	})
}

func (CorporateFinanceBuilder) Decide(opts provider.RegisterOptions, config provider.Config) provider.RegistrationDecision {
	return decideProvider(decideProviderInput{
		ID:           provider.ProviderDataGoCorporateFinance,
		Label:        "datago-corpfin",
		Options:      opts,
		Config:       config,
		ServiceKey:   corporateFinanceServiceKeyFromConfig(config),
		GroupEnabled: corporateFinanceEnabledFromConfig(config),
		GroupReason:  "datago-corpfin corporateFinance group disabled",
	})
}

func (Builder) Build(config provider.Config) (provider.IdentityProvider, error) {
	serviceKey := serviceKeyFromConfig(config)
	if serviceKey == "" {
		return nil, missingServiceKeyError(provider.ProviderDataGo, "providers.datago.auth.service_key", serviceKeyEnv, serviceKeyFallbackEnv)
	}
	return New(Config{
		ServiceKey:       serviceKey,
		BaseURL:          baseURLFromConfig(config),
		RetryMaxAttempts: 0,
	})
}

func (CorporateFinanceBuilder) Build(config provider.Config) (provider.IdentityProvider, error) {
	serviceKey := corporateFinanceServiceKeyFromConfig(config)
	if serviceKey == "" {
		return nil, missingServiceKeyError(provider.ProviderDataGoCorporateFinance, "providers.datago-corpfin.auth.service_key", corporateFinanceServiceKeyEnv, serviceKeyEnv, serviceKeyFallbackEnv)
	}
	listedInfoKey := krxListedInfoServiceKeyFromConfig(config)
	if listedInfoKey == "" {
		return nil, missingServiceKeyError(provider.ProviderDataGoCorporateFinance, "providers.datago-corpfin.dependencies.krxListedInfo.auth.service_key", krxListedInfoServiceKeyEnv, serviceKeyEnv, serviceKeyFallbackEnv)
	}
	return NewCorporateFinance(CorporateFinanceConfig{
		ServiceKey:              serviceKey,
		CorporateFinanceBaseURL: corporateFinanceBaseURLFromConfig(config),
		KRXListedInfoServiceKey: listedInfoKey,
		KRXListedInfoBaseURL:    krxListedInfoBaseURLFromConfig(config),
		RetryMaxAttempts:        0,
	})
}

type decideProviderInput struct {
	ID           provider.ProviderID
	Label        string
	Options      provider.RegisterOptions
	Config       provider.Config
	ServiceKey   string
	GroupEnabled bool
	GroupReason  string
}

func decideProvider(input decideProviderInput) provider.RegistrationDecision {
	if !enabledFromConfig(input.Config, input.ID) {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   input.Label + " disabled",
		}
	}
	requested := requestsProvider(input.Options, input.ID)
	if forcedOtherProvider(input.Options, input.ID) && !requested {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   "another provider is forced",
		}
	}
	if !input.GroupEnabled {
		return provider.RegistrationDecision{
			Register: false,
			Reason:   input.GroupReason,
		}
	}
	if input.ServiceKey != "" {
		return provider.RegistrationDecision{
			Register: true,
			Reason:   input.Label + " config is present",
		}
	}
	if requested {
		return provider.RegistrationDecision{
			Register: true,
			Reason:   input.Label + " requested",
		}
	}
	return provider.RegistrationDecision{
		Register: false,
		Reason:   input.Label + " config missing",
	}
}

func missingServiceKeyError(id provider.ProviderID, path string, envs ...string) error {
	return oops.In("provider_registry").
		With("provider", id).
		Errorf("%s provider config requires service key: configure %s or set %s", id, path, strings.Join(envs, " or "))
}

func serviceKeyFromConfig(config provider.Config) string {
	return stringFromConfigOrEnv(config, []string{"providers", string(provider.ProviderDataGo), "auth", "service_key"}, serviceKeyEnv, serviceKeyFallbackEnv)
}

func corporateFinanceServiceKeyFromConfig(config provider.Config) string {
	return stringFromConfigOrEnv(config, []string{"providers", string(provider.ProviderDataGoCorporateFinance), "auth", "service_key"}, corporateFinanceServiceKeyEnv, serviceKeyEnv, serviceKeyFallbackEnv)
}

func krxListedInfoServiceKeyFromConfig(config provider.Config) string {
	return stringFromConfigOrEnv(config, []string{"providers", string(provider.ProviderDataGoCorporateFinance), "dependencies", string(provider.GroupKRXListedInfo), "auth", "service_key"}, krxListedInfoServiceKeyEnv, serviceKeyEnv, serviceKeyFallbackEnv)
}

func baseURLFromConfig(config provider.Config) string {
	return stringFromConfigOrEnv(config, []string{"providers", string(provider.ProviderDataGo), "base_url"}, baseURLEnv)
}

func corporateFinanceBaseURLFromConfig(config provider.Config) string {
	return stringFromConfigOrEnv(config, []string{"providers", string(provider.ProviderDataGoCorporateFinance), "base_url"}, corporateFinanceBaseURLEnv)
}

func krxListedInfoBaseURLFromConfig(config provider.Config) string {
	return stringFromConfigOrEnv(config, []string{"providers", string(provider.ProviderDataGoCorporateFinance), "dependencies", string(provider.GroupKRXListedInfo), "base_url"}, krxListedInfoBaseURLEnv)
}

func stringFromConfigOrEnv(config provider.Config, path []string, envs ...string) string {
	value := strings.TrimSpace(config.String(path...))
	if value != "" {
		return value
	}
	for _, env := range envs {
		value = strings.TrimSpace(config.Env(env))
		if value != "" {
			return value
		}
	}
	return ""
}

func enabledFromConfig(config provider.Config, id provider.ProviderID) bool {
	enabled, ok := config.Bool("providers", string(id), "enabled")
	return !ok || enabled
}

func securitiesProductPriceEnabledFromConfig(config provider.Config) bool {
	enabled, ok := config.Bool("providers", string(provider.ProviderDataGo), "groups", string(provider.GroupSecuritiesProductPrice), "enabled")
	return !ok || enabled
}

func corporateFinanceEnabledFromConfig(config provider.Config) bool {
	enabled, ok := config.Bool("providers", string(provider.ProviderDataGoCorporateFinance), "groups", string(provider.GroupCorporateFinance), "enabled")
	return !ok || enabled
}

func requestsProvider(opts provider.RegisterOptions, id provider.ProviderID) bool {
	return opts.ProviderID == id || opts.PreferProvider == id
}

func forcedOtherProvider(opts provider.RegisterOptions, id provider.ProviderID) bool {
	return opts.ProviderID != "" && opts.ProviderID != id
}
