package core

import "github.com/samber/oops"

type RoleProfile struct {
	Role          Role
	Markets       []Market
	SecurityTypes []SecurityType
	Group         GroupID
	Operations    []OperationID
	AuthScope     CredentialScope
	Freshness     Freshness
	RequiresAuth  bool
	Priority      int
	Limitations   []string
}

type RoleRegistration struct {
	Profile RoleProfile
	Impl    any
}

type RoleEntry struct {
	Provider Identity
	Profile  RoleProfile
	Impl     any
}

type Registry struct {
	entries []RoleEntry
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(provider IdentityProvider, roles ...RoleRegistration) error {
	if provider == nil {
		return oops.In("provider_registry").New("register provider role: provider identity is nil")
	}
	identity := provider.ProviderIdentity()
	if identity.ID == "" {
		return oops.In("provider_registry").New("register provider role: provider id is empty")
	}
	for _, role := range roles {
		if role.Profile.Role == "" {
			return oops.In("provider_registry").With("provider", identity.ID).New("register provider role: role is empty")
		}
		if role.Impl == nil {
			return oops.In("provider_registry").With("provider", identity.ID, "role", role.Profile.Role).New("register provider role: implementation is nil")
		}
		r.entries = append(r.entries, RoleEntry{
			Provider: identity,
			Profile:  role.Profile,
			Impl:     role.Impl,
		})
	}
	return nil
}

func (r *Registry) Entries(role Role) []RoleEntry {
	entries := make([]RoleEntry, 0)
	for _, entry := range r.entries {
		if entry.Profile.Role == role {
			entries = append(entries, entry)
		}
	}
	return entries
}
