package core

import "fmt"

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
		return fmt.Errorf("register provider role: provider identity is nil")
	}
	identity := provider.ProviderIdentity()
	if identity.ID == "" {
		return fmt.Errorf("register provider role: provider id is empty")
	}
	for _, role := range roles {
		if role.Profile.Role == "" {
			return fmt.Errorf("register provider role: role is empty provider=%s", identity.ID)
		}
		if role.Impl == nil {
			return fmt.Errorf("register provider role: implementation is nil provider=%s role=%s", identity.ID, role.Profile.Role)
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
