package quote

import (
	"context"
	"fmt"

	provider "github.com/ev3rlit/mwosa/providers/core"
)

type RouteInput struct {
	ProviderID     provider.ProviderID
	PreferProvider provider.ProviderID
	Market         provider.Market
	SecurityType   provider.SecurityType
	Group          provider.GroupID
	Operation      provider.OperationID
	Symbol         string
}

type RoutePlan struct {
	Candidates []RouteCandidate
}

type RouteCandidate struct {
	Provider    provider.Identity
	Group       provider.GroupID
	Snapshotter Snapshotter
	Profile     Profile
	Reason      string
}

type Router interface {
	RouteQuoteSnapshot(ctx context.Context, input RouteInput) (Snapshotter, error)
	PlanQuoteSnapshot(ctx context.Context, input RouteInput) (RoutePlan, error)
}

type coreRouter interface {
	Route(context.Context, provider.RouteInput) (provider.RouteCandidate, error)
	Plan(context.Context, provider.RouteInput) (provider.RoutePlan, error)
}

type routeAdapter struct {
	router coreRouter
}

func NewRouter(router coreRouter) Router {
	return routeAdapter{router: router}
}

func (r routeAdapter) RouteQuoteSnapshot(ctx context.Context, input RouteInput) (Snapshotter, error) {
	candidate, err := r.router.Route(ctx, toCoreRouteInput(input))
	if err != nil {
		return nil, err
	}
	snapshotter, ok := candidate.Impl.(Snapshotter)
	if !ok {
		return nil, fmt.Errorf("routed quote implementation does not satisfy Snapshotter provider=%s", candidate.Provider.ID)
	}
	return snapshotter, nil
}

func (r routeAdapter) PlanQuoteSnapshot(ctx context.Context, input RouteInput) (RoutePlan, error) {
	plan, err := r.router.Plan(ctx, toCoreRouteInput(input))
	if err != nil {
		return RoutePlan{}, err
	}
	candidates := make([]RouteCandidate, 0, len(plan.Candidates))
	for _, candidate := range plan.Candidates {
		snapshotter, ok := candidate.Impl.(Snapshotter)
		if !ok {
			return RoutePlan{}, fmt.Errorf("routed quote implementation does not satisfy Snapshotter provider=%s", candidate.Provider.ID)
		}
		candidates = append(candidates, RouteCandidate{
			Provider:    candidate.Provider,
			Group:       candidate.Profile.Group,
			Snapshotter: snapshotter,
			Profile:     snapshotter.QuoteProfile(),
			Reason:      candidate.Reason,
		})
	}
	return RoutePlan{Candidates: candidates}, nil
}

func toCoreRouteInput(input RouteInput) provider.RouteInput {
	return provider.RouteInput{
		Role:           provider.RoleQuote,
		ProviderID:     input.ProviderID,
		PreferProvider: input.PreferProvider,
		Market:         input.Market,
		SecurityType:   input.SecurityType,
		Group:          input.Group,
		Operation:      input.Operation,
		Symbol:         input.Symbol,
	}
}
