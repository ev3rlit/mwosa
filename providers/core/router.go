package core

import (
	"context"
	"errors"
	"sort"

	"github.com/samber/oops"
)

type RouteInput struct {
	Role           Role
	ProviderID     ProviderID
	PreferProvider ProviderID
	Market         Market
	SecurityType   SecurityType
	Group          GroupID
	Operation      OperationID
	Symbol         string
}

type RoutePlan struct {
	Candidates []RouteCandidate
}

type RouteCandidate struct {
	Provider Identity
	Profile  RoleProfile
	Impl     any
	Reason   string
}

type Router struct {
	registry *Registry
}

func NewRouter(registry *Registry) *Router {
	return &Router{registry: registry}
}

func (r *Router) Route(ctx context.Context, input RouteInput) (RouteCandidate, error) {
	plan, err := r.Plan(ctx, input)
	if err != nil {
		return RouteCandidate{}, oops.In("provider_router").With("role", input.Role, "market", input.Market, "security_type", input.SecurityType, "provider", input.ProviderID, "symbol", input.Symbol).Wrap(err)
	}
	if len(plan.Candidates) == 0 {
		return RouteCandidate{}, oops.In("provider_router").With("role", input.Role, "market", input.Market, "security_type", input.SecurityType, "provider", input.ProviderID, "symbol", input.Symbol).Wrapf(ErrNoProvider, "role=%s market=%s security_type=%s provider=%s symbol=%s", input.Role, input.Market, input.SecurityType, input.ProviderID, input.Symbol)
	}
	return plan.Candidates[0], nil
}

func (r *Router) Plan(ctx context.Context, input RouteInput) (RoutePlan, error) {
	if err := ctx.Err(); err != nil {
		return RoutePlan{}, oops.In("provider_router").With("role", input.Role).Wrap(err)
	}
	if r == nil || r.registry == nil {
		return RoutePlan{}, oops.In("provider_router").With("role", input.Role, "reason", "registry is nil").Wrapf(ErrNoProvider, "registry is nil role=%s", input.Role)
	}

	candidates := make([]RouteCandidate, 0)
	for _, entry := range r.registry.Entries(input.Role) {
		if input.ProviderID != "" && entry.Provider.ID != input.ProviderID {
			continue
		}
		if input.Market != "" && !containsMarket(entry.Profile.Markets, input.Market) {
			continue
		}
		if input.SecurityType != "" && !containsSecurityType(entry.Profile.SecurityTypes, input.SecurityType) {
			continue
		}
		if input.Group != "" && entry.Profile.Group != input.Group {
			continue
		}
		if input.Operation != "" && !containsOperation(entry.Profile.Operations, input.Operation) {
			continue
		}
		candidates = append(candidates, RouteCandidate{
			Provider: entry.Provider,
			Profile:  entry.Profile,
			Impl:     entry.Impl,
			Reason:   "profile matched",
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if input.PreferProvider != "" {
			leftPreferred := candidates[i].Provider.ID == input.PreferProvider
			rightPreferred := candidates[j].Provider.ID == input.PreferProvider
			if leftPreferred != rightPreferred {
				return leftPreferred
			}
		}
		return candidates[i].Profile.Priority > candidates[j].Profile.Priority
	})

	if len(candidates) == 0 {
		return RoutePlan{}, oops.In("provider_router").With("role", input.Role, "market", input.Market, "security_type", input.SecurityType, "provider", input.ProviderID, "group", input.Group, "operation", input.Operation, "symbol", input.Symbol).Wrapf(ErrNoProvider, "role=%s market=%s security_type=%s provider=%s group=%s operation=%s symbol=%s", input.Role, input.Market, input.SecurityType, input.ProviderID, input.Group, input.Operation, input.Symbol)
	}

	return RoutePlan{Candidates: candidates}, nil
}

func IsNoProvider(err error) bool {
	return errors.Is(err, ErrNoProvider)
}

func containsMarket(values []Market, target Market) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsSecurityType(values []SecurityType, target SecurityType) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsOperation(values []OperationID, target OperationID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
