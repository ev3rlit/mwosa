package core

type RegisterOptions struct {
	ProviderID     ProviderID
	PreferProvider ProviderID
}

type RegistrationDecision struct {
	Register bool
	Reason   string
}

type ProviderBuilder interface {
	ID() ProviderID
	DefaultConfig() Config
	ConfigSpec() ConfigSpec
	Decide(RegisterOptions, Config) RegistrationDecision
	Build(Config) (IdentityProvider, error)
}
