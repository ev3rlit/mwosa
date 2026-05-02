package instrument

import (
	"context"
	"fmt"
	"strings"

	provider "github.com/ev3rlit/mwosa/providers/core"
	instrumentrole "github.com/ev3rlit/mwosa/providers/core/instrument"
	"github.com/samber/oops"
)

type Router interface {
	RouteInstrumentSearch(ctx context.Context, input instrumentrole.RouteInput) (instrumentrole.Searcher, error)
}

type Service struct {
	router Router
}

const inspectSearchLimit = 25

func NewService(router Router) (Service, error) {
	if router == nil {
		return Service{}, oops.In("instrument_service").New("instrument service router is nil")
	}
	return Service{router: router}, nil
}

type SearchRequest struct {
	ProviderID     provider.ProviderID
	PreferProvider provider.ProviderID
	Market         provider.Market
	SecurityType   provider.SecurityType
	Query          string
	Limit          int
}

type InspectRequest struct {
	ProviderID     provider.ProviderID
	PreferProvider provider.ProviderID
	Market         provider.Market
	SecurityType   provider.SecurityType
	Symbol         string
}

type InspectResult struct {
	Instrument instrumentrole.Instrument
	Provider   provider.Identity
	Group      provider.GroupID
	Operations []provider.OperationID
}

type NotFoundError struct {
	Query        string
	Market       provider.Market
	SecurityType provider.SecurityType
}

func (e *NotFoundError) Error() string {
	parts := []string{"instrument not found"}
	if e.Market != "" {
		parts = append(parts, fmt.Sprintf("market=%s", e.Market))
	}
	if e.SecurityType != "" {
		parts = append(parts, fmt.Sprintf("security_type=%s", e.SecurityType))
	}
	if e.Query != "" {
		parts = append(parts, fmt.Sprintf("query=%s", e.Query))
	}
	return strings.Join(parts, " ")
}

func (s Service) Search(ctx context.Context, req SearchRequest) (instrumentrole.SearchResult, error) {
	errb := oops.In("instrument_service").With("provider", req.ProviderID, "prefer_provider", req.PreferProvider, "market", req.Market, "security_type", req.SecurityType, "query", req.Query)
	if req.Query == "" {
		return instrumentrole.SearchResult{}, errb.New("search instruments requires query")
	}
	if s.router == nil {
		return instrumentrole.SearchResult{}, errb.New("instrument service router is nil")
	}

	searcher, err := s.router.RouteInstrumentSearch(ctx, instrumentrole.RouteInput{
		ProviderID:     req.ProviderID,
		PreferProvider: req.PreferProvider,
		Market:         req.Market,
		SecurityType:   req.SecurityType,
		Symbol:         req.Query,
	})
	if err != nil {
		return instrumentrole.SearchResult{}, errb.Wrapf(err, "route instrument search")
	}

	result, err := searcher.SearchInstruments(ctx, instrumentrole.SearchInput{
		Market:       req.Market,
		SecurityType: req.SecurityType,
		Query:        req.Query,
		Limit:        req.Limit,
	})
	if err != nil {
		return instrumentrole.SearchResult{}, errb.With("provider", req.ProviderID).Wrapf(err, "search instruments")
	}
	return result, nil
}

func (s Service) Inspect(ctx context.Context, req InspectRequest) (InspectResult, error) {
	symbol := strings.TrimSpace(req.Symbol)
	errb := oops.In("instrument_service").With("provider", req.ProviderID, "prefer_provider", req.PreferProvider, "market", req.Market, "security_type", req.SecurityType, "symbol", symbol)
	if symbol == "" {
		return InspectResult{}, errb.New("inspect instrument requires symbol")
	}

	result, err := s.Search(ctx, SearchRequest{
		ProviderID:     req.ProviderID,
		PreferProvider: req.PreferProvider,
		Market:         req.Market,
		SecurityType:   req.SecurityType,
		Query:          symbol,
		Limit:          inspectSearchLimit,
	})
	if err != nil {
		return InspectResult{}, errb.Wrap(err)
	}
	for _, candidate := range result.Instruments {
		if !matchesInstrumentIdentity(candidate, symbol) {
			continue
		}
		return InspectResult{
			Instrument: candidate,
			Provider:   result.Provider,
			Group:      result.Group,
			Operations: result.Operations,
		}, nil
	}

	return InspectResult{}, errb.Wrap(&NotFoundError{
		Query:        symbol,
		Market:       req.Market,
		SecurityType: req.SecurityType,
	})
}

func matchesInstrumentIdentity(candidate instrumentrole.Instrument, symbol string) bool {
	return matchesInstrumentIdentifier(candidate.SecurityCode, symbol) ||
		matchesInstrumentIdentifier(candidate.ISIN, symbol) ||
		matchesInstrumentIdentifier(candidate.ExchangeCode, symbol)
}

func matchesInstrumentIdentifier(value string, symbol string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return strings.EqualFold(value, symbol)
}
