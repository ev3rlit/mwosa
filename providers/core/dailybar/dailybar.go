package dailybar

import (
	"context"
	"fmt"

	provider "github.com/ev3rlit/mwosa/providers/core"
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
}

type Bar struct {
	Provider     provider.ProviderID
	Group        provider.GroupID
	Operation    provider.OperationID
	Market       provider.Market
	SecurityType provider.SecurityType

	Symbol      string
	ISIN        string
	Name        string
	TradingDate string
	Currency    string

	Open        string
	High        string
	Low         string
	Close       string
	Change      string
	ChangeRate  string
	Volume      string
	TradedValue string
	MarketCap   string

	Extensions map[string]string
}

type FetchResult struct {
	Bars       []Bar
	Provider   provider.Identity
	Group      provider.GroupID
	Operation  provider.OperationID
	TotalCount int
}

type Fetcher interface {
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
		return FetchResult{}, fmt.Errorf("dailybar fetch role is not configured")
	}
	return f.fetch(ctx, input)
}

func (f Fetch) DailyBarProfile() Profile {
	return f.profile
}
