package dailybar

import (
	"context"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
)

type RangeQuerySupport string

const (
	RangeQueryUnsupported RangeQuerySupport = "unsupported"
	RangeQuerySupported   RangeQuerySupport = "supported"
)

type Profile struct {
	Markets       []provider.Market
	SecurityTypes []provider.SecurityType
	Group         provider.GroupID
	Operations    []provider.OperationID
	AuthScope     provider.CredentialScope
	RangeQuery    RangeQuerySupport
	Freshness     provider.Freshness
	Compatibility provider.Compatibility
	RequiresAuth  bool
	Priority      int
	Limitations   []string
}

func (p Profile) RoleProfile() provider.RoleProfile {
	return provider.RoleProfile{
		Role:          provider.RoleDailyBar,
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

type FetchInput struct {
	Market       provider.Market
	SecurityType provider.SecurityType
	Symbol       string
	From         string
	To           string
	Limit        int
	Workers      int
}

type Bar struct {
	Provider     provider.ProviderID   `json:"provider"`
	Group        provider.GroupID      `json:"provider_group"`
	Operation    provider.OperationID  `json:"operation"`
	Market       provider.Market       `json:"market"`
	SecurityType provider.SecurityType `json:"security_type"`

	Symbol      string `json:"symbol"`
	ISIN        string `json:"isin,omitempty"`
	Name        string `json:"name,omitempty"`
	TradingDate string `json:"trading_date"`
	Currency    string `json:"currency"`

	Open        string `json:"opening_price,omitempty"`
	High        string `json:"highest_price,omitempty"`
	Low         string `json:"lowest_price,omitempty"`
	Close       string `json:"closing_price,omitempty"`
	Change      string `json:"price_change_from_previous_close,omitempty"`
	ChangeRate  string `json:"price_change_rate_from_previous_close,omitempty"`
	Volume      string `json:"traded_volume,omitempty"`
	TradedValue string `json:"traded_amount,omitempty"`
	MarketCap   string `json:"market_capitalization,omitempty"`

	Extensions map[string]string `json:"extensions,omitempty"`
}

type FetchResult struct {
	Bars       []Bar
	Provider   provider.Identity
	Group      provider.GroupID
	Operation  provider.OperationID
	TotalCount int
}

type Fetcher interface {
	provider.RoleProvider
	FetchDailyBars(ctx context.Context, input FetchInput) (FetchResult, error)
	DailyBarProfile() Profile
}

type FetchFunc func(context.Context, FetchInput) (FetchResult, error)

type Fetch struct {
	profile Profile
	fetch   FetchFunc
}

func NewFetch(profile Profile, fetch FetchFunc) Fetch {
	return Fetch{profile: profile, fetch: fetch}
}

func (f Fetch) FetchDailyBars(ctx context.Context, input FetchInput) (FetchResult, error) {
	if f.fetch == nil {
		return FetchResult{}, oops.In("provider_role").With("role", provider.RoleDailyBar).New("dailybar fetch role is not configured")
	}
	return f.fetch(ctx, input)
}

func (f Fetch) DailyBarProfile() Profile {
	return f.profile
}

func (f Fetch) RoleRegistration() provider.RoleRegistration {
	return provider.RoleRegistration{
		Profile: f.profile.RoleProfile(),
		Impl:    f,
	}
}
