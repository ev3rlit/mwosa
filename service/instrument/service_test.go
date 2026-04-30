package instrument

import (
	"context"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	instrumentrole "github.com/ev3rlit/mwosa/providers/core/instrument"
)

func TestNewServiceRequiresRouter(t *testing.T) {
	_, err := NewService(nil)
	if err == nil {
		t.Fatal("NewService error = nil, want router error")
	}
	if !strings.Contains(err.Error(), "router") {
		t.Fatalf("error = %q, want router context", err.Error())
	}
}

func TestSearchRoutesAndCallsInstrumentSearcher(t *testing.T) {
	var gotSearch instrumentrole.SearchInput
	searcher := instrumentrole.NewSearch(instrumentrole.Profile{}, func(_ context.Context, input instrumentrole.SearchInput) (instrumentrole.SearchResult, error) {
		gotSearch = input
		return instrumentrole.SearchResult{
			Instruments: []instrumentrole.Instrument{
				{
					Provider:     provider.ProviderID("fake"),
					Market:       provider.MarketKRX,
					SecurityType: provider.SecurityTypeETF,
					SecurityCode: "069500",
					Name:         "KODEX 200",
				},
			},
			Provider: provider.Identity{ID: provider.ProviderID("fake")},
			Group:    provider.GroupID("fakeGroup"),
		}, nil
	})
	router := &fakeInstrumentRouter{searcher: searcher}
	service, err := NewService(router)
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}

	result, err := service.Search(context.Background(), SearchRequest{
		ProviderID:   provider.ProviderID("fake"),
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Query:        "069500",
		Limit:        5,
	})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}

	if router.gotRoute.ProviderID != provider.ProviderID("fake") || router.gotRoute.Symbol != "069500" {
		t.Fatalf("route input = %+v, want provider and symbol", router.gotRoute)
	}
	if gotSearch.Query != "069500" || gotSearch.Limit != 5 {
		t.Fatalf("search input = %+v, want query and limit", gotSearch)
	}
	if len(result.Instruments) != 1 {
		t.Fatalf("instruments len = %d, want 1", len(result.Instruments))
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	service, err := NewService(&fakeInstrumentRouter{})
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}

	_, err = service.Search(context.Background(), SearchRequest{})
	if err == nil {
		t.Fatal("Search error = nil, want query error")
	}
	if !strings.Contains(err.Error(), "requires query") {
		t.Fatalf("error = %q, want query context", err.Error())
	}
}

func TestInspectReturnsFirstMatchedInstrument(t *testing.T) {
	searcher := instrumentrole.NewSearch(instrumentrole.Profile{}, func(_ context.Context, input instrumentrole.SearchInput) (instrumentrole.SearchResult, error) {
		if input.Limit != 1 {
			t.Fatalf("inspect search limit = %d, want 1", input.Limit)
		}
		return instrumentrole.SearchResult{
			Instruments: []instrumentrole.Instrument{
				{SecurityCode: "069500", Name: "KODEX 200"},
			},
			Provider:   provider.Identity{ID: provider.ProviderID("fake")},
			Group:      provider.GroupID("fakeGroup"),
			Operations: []provider.OperationID{provider.OperationID("fakeOperation")},
		}, nil
	})
	service, err := NewService(&fakeInstrumentRouter{searcher: searcher})
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}

	result, err := service.Inspect(context.Background(), InspectRequest{Symbol: "069500"})
	if err != nil {
		t.Fatalf("Inspect error = %v", err)
	}
	if result.Instrument.SecurityCode != "069500" || result.Provider.ID != provider.ProviderID("fake") {
		t.Fatalf("inspect result = %+v", result)
	}
}

func TestInspectReportsNotFound(t *testing.T) {
	searcher := instrumentrole.NewSearch(instrumentrole.Profile{}, func(context.Context, instrumentrole.SearchInput) (instrumentrole.SearchResult, error) {
		return instrumentrole.SearchResult{}, nil
	})
	service, err := NewService(&fakeInstrumentRouter{searcher: searcher})
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}

	_, err = service.Inspect(context.Background(), InspectRequest{Symbol: "missing"})
	if err == nil {
		t.Fatal("Inspect error = nil, want not found")
	}
	if !strings.Contains(err.Error(), "instrument not found") {
		t.Fatalf("error = %q, want not found context", err.Error())
	}
}

type fakeInstrumentRouter struct {
	searcher instrumentrole.Searcher
	gotRoute instrumentrole.RouteInput
}

func (r *fakeInstrumentRouter) RouteInstrumentSearch(_ context.Context, input instrumentrole.RouteInput) (instrumentrole.Searcher, error) {
	r.gotRoute = input
	return r.searcher, nil
}
