package dailybar

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
	Provider provider.Identity
	Group    provider.GroupID
	Fetcher  Fetcher
	Profile  Profile
	Reason   string
}

type Router interface {
	RouteDailyBars(ctx context.Context, input RouteInput) (Fetcher, error)
	PlanDailyBars(ctx context.Context, input RouteInput) (RoutePlan, error)
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

func (r routeAdapter) RouteDailyBars(ctx context.Context, input RouteInput) (Fetcher, error) {
	candidate, err := r.router.Route(ctx, toCoreRouteInput(input))
	if err != nil {
		return nil, err
	}
	fetcher, ok := candidate.Impl.(Fetcher)
	if !ok {
		return nil, fmt.Errorf("routed dailybar implementation does not satisfy Fetcher provider=%s", candidate.Provider.ID)
	}
	return fetcher, nil
}

func (r routeAdapter) PlanDailyBars(ctx context.Context, input RouteInput) (RoutePlan, error) {
	plan, err := r.router.Plan(ctx, toCoreRouteInput(input))
	if err != nil {
		return RoutePlan{}, err
	}
	candidates := make([]RouteCandidate, 0, len(plan.Candidates))
	for _, candidate := range plan.Candidates {
		fetcher, ok := candidate.Impl.(Fetcher)
		if !ok {
			return RoutePlan{}, fmt.Errorf("routed dailybar implementation does not satisfy Fetcher provider=%s", candidate.Provider.ID)
		}
		candidates = append(candidates, RouteCandidate{
			Provider: candidate.Provider,
			Group:    candidate.Profile.Group,
			Fetcher:  fetcher,
			Profile:  fetcher.DailyBarProfile(),
			Reason:   candidate.Reason,
		})
	}
	return RoutePlan{Candidates: candidates}, nil
}

func toCoreRouteInput(input RouteInput) provider.RouteInput {
	return provider.RouteInput{
		Role:           provider.RoleDailyBar,
		ProviderID:     input.ProviderID,
		PreferProvider: input.PreferProvider,
		Market:         input.Market,
		SecurityType:   input.SecurityType,
		Group:          input.Group,
		Operation:      input.Operation,
		Symbol:         input.Symbol,
	}
}
