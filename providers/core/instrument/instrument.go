package instrument

import (
	"context"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
)

type Profile struct {
	Markets       []provider.Market
	SecurityTypes []provider.SecurityType
	Group         provider.GroupID
	Operations    []provider.OperationID
	AuthScope     provider.CredentialScope
	Freshness     provider.Freshness
	Compatibility provider.Compatibility
	RequiresAuth  bool
	Priority      int
	Limitations   []string
}

func (p Profile) RoleProfile() provider.RoleProfile {
	return provider.RoleProfile{
		Role:          provider.RoleInstrument,
		Markets:       p.Markets,
		SecurityTypes: p.SecurityTypes,
		Group:         p.Group,
		Operations:    p.Operations,
		AuthScope:     p.AuthScope,
		Freshness:     p.Freshness,
		Compatibility: p.Compatibility,
		RequiresAuth:  p.RequiresAuth,
		Priority:      p.Priority,
		Limitations:   p.Limitations,
	}
}

type SearchInput struct {
	Market       provider.Market
	SecurityType provider.SecurityType
	Query        string
	Limit        int
}

type Instrument struct {
	Provider     provider.ProviderID
	Group        provider.GroupID
	Operation    provider.OperationID
	Market       provider.Market
	SecurityType provider.SecurityType

	SecurityCode string
	ISIN         string
	Name         string
	ExchangeCode string
	CountryCode  string
	Timezone     string

	Extensions map[string]string
}

type SearchResult struct {
	Instruments []Instrument
	Provider    provider.Identity
	Group       provider.GroupID
	Operations  []provider.OperationID
	TotalCount  int
}

type Searcher interface {
	provider.RoleProvider
	SearchInstruments(ctx context.Context, input SearchInput) (SearchResult, error)
	InstrumentSearchProfile() Profile
}

type SearchFunc func(context.Context, SearchInput) (SearchResult, error)

type Search struct {
	profile Profile
	search  SearchFunc
}

func NewSearch(profile Profile, search SearchFunc) Search {
	return Search{profile: profile, search: search}
}

func (s Search) SearchInstruments(ctx context.Context, input SearchInput) (SearchResult, error) {
	if s.search == nil {
		return SearchResult{}, oops.In("provider_role").With("role", provider.RoleInstrument).New("instrument search role is not configured")
	}
	return s.search(ctx, input)
}

func (s Search) InstrumentSearchProfile() Profile {
	return s.profile
}

func (s Search) RoleRegistration() provider.RoleRegistration {
	return provider.RoleRegistration{
		Profile: s.profile.RoleProfile(),
		Impl:    s,
	}
}
