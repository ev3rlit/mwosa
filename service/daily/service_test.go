package daily

import (
	"context"
	"strings"
	"testing"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
)

func TestNewReadServiceRequiresReader(t *testing.T) {
	_, err := NewReadService(nil)
	if err == nil {
		t.Fatal("NewReadService error = nil, want reader error")
	}
	if !strings.Contains(err.Error(), "read repository") {
		t.Fatalf("error = %q, want read repository context", err.Error())
	}
}

func TestNewServiceRequiresDependencies(t *testing.T) {
	tests := []struct {
		name   string
		reader ReadRepository
		writer WriteRepository
		router dailybar.Router
		want   string
	}{
		{
			name:   "reader",
			writer: fakeWriteRepository{},
			router: fakeDailyBarRouter{},
			want:   "read repository",
		},
		{
			name:   "writer",
			reader: fakeReadRepository{},
			router: fakeDailyBarRouter{},
			want:   "write repository",
		},
		{
			name:   "router",
			reader: fakeReadRepository{},
			writer: fakeWriteRepository{},
			want:   "router",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(tt.reader, tt.writer, tt.router)
			if err == nil {
				t.Fatal("NewService error = nil, want dependency error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNewServiceAcceptsInjectedRouter(t *testing.T) {
	_, err := NewService(fakeReadRepository{}, fakeWriteRepository{}, fakeDailyBarRouter{})
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}
}

func TestEnsurePassesSymbolToProviderFetch(t *testing.T) {
	reader := &sequenceReadRepository{
		results: [][]dailybar.Bar{
			nil,
			{{Symbol: "069500", TradingDate: "2024-04-15"}},
		},
	}
	var gotFetch dailybar.FetchInput
	fetcher := dailybar.NewFetch(dailybar.Profile{
		Markets:       []provider.Market{provider.MarketKRX},
		SecurityTypes: []provider.SecurityType{provider.SecurityTypeETF},
		Compatibility: provider.Compatibility{DataLatency: provider.DataLatencyPreviousBusinessDay},
	}, func(_ context.Context, input dailybar.FetchInput) (dailybar.FetchResult, error) {
		gotFetch = input
		return dailybar.FetchResult{
			Bars: []dailybar.Bar{
				{Symbol: "069500", TradingDate: "2024-04-15"},
			},
			Provider: provider.Identity{ID: provider.ProviderID("fake")},
		}, nil
	})

	service, err := NewService(reader, fakeWriteRepository{}, fakeDailyBarRouter{fetcher: fetcher})
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}

	_, err = service.Ensure(context.Background(), Request{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		Symbol:       "069500",
		AsOf:         "20240415",
	})
	if err != nil {
		t.Fatalf("Ensure error = %v", err)
	}
	if gotFetch.Symbol != "069500" {
		t.Fatalf("fetch symbol = %q, want 069500", gotFetch.Symbol)
	}
}

func TestBackfillAcceptsWorkers(t *testing.T) {
	fetcher := dailybar.NewFetch(dailybar.Profile{
		Markets:       []provider.Market{provider.MarketKRX},
		SecurityTypes: []provider.SecurityType{provider.SecurityTypeETF},
		Compatibility: provider.Compatibility{DataLatency: provider.DataLatencyPreviousBusinessDay},
	}, func(_ context.Context, input dailybar.FetchInput) (dailybar.FetchResult, error) {
		return dailybar.FetchResult{
			Bars: []dailybar.Bar{
				{Symbol: "069500", TradingDate: input.From},
			},
			Provider: provider.Identity{ID: provider.ProviderID("fake")},
			Group:    provider.GroupID("fakeGroup"),
		}, nil
	})
	writer := &recordingWriteRepository{}
	service, err := NewService(fakeReadRepository{}, writer, fakeDailyBarRouter{fetcher: fetcher})
	if err != nil {
		t.Fatalf("NewService error = %v", err)
	}

	result, err := service.Backfill(context.Background(), Request{
		Market:       provider.MarketKRX,
		SecurityType: provider.SecurityTypeETF,
		From:         "20240415",
		To:           "20240416",
		Workers:      2,
	})
	if err != nil {
		t.Fatalf("Backfill error = %v", err)
	}
	if result.BarsFetched != 2 || result.BarsStored != 2 {
		t.Fatalf("result = %+v, want fetched/stored 2", result)
	}
	if strings.Join(result.Dates, ",") != "2024-04-15,2024-04-16" {
		t.Fatalf("dates = %v, want sorted backfill dates", result.Dates)
	}
	if writer.barsWritten != 2 {
		t.Fatalf("writer bars = %d, want 2", writer.barsWritten)
	}
}

type fakeReadRepository struct{}

func (fakeReadRepository) QueryDailyBars(context.Context, Query) ([]dailybar.Bar, error) {
	return nil, nil
}

type fakeWriteRepository struct{}

func (fakeWriteRepository) UpsertDailyBars(context.Context, []dailybar.Bar) (WriteResult, error) {
	return WriteResult{}, nil
}

type recordingWriteRepository struct {
	barsWritten int
}

func (r *recordingWriteRepository) UpsertDailyBars(_ context.Context, bars []dailybar.Bar) (WriteResult, error) {
	r.barsWritten += len(bars)
	return WriteResult{BarsWritten: len(bars), RowsAffected: len(bars)}, nil
}

type fakeDailyBarRouter struct {
	fetcher dailybar.Fetcher
}

func (r fakeDailyBarRouter) RouteDailyBars(context.Context, dailybar.RouteInput) (dailybar.Fetcher, error) {
	return r.fetcher, nil
}

func (fakeDailyBarRouter) PlanDailyBars(context.Context, dailybar.RouteInput) (dailybar.RoutePlan, error) {
	return dailybar.RoutePlan{}, nil
}

type sequenceReadRepository struct {
	results [][]dailybar.Bar
	calls   int
}

func (r *sequenceReadRepository) QueryDailyBars(context.Context, Query) ([]dailybar.Bar, error) {
	if r.calls >= len(r.results) {
		return nil, nil
	}
	result := r.results[r.calls]
	r.calls++
	return result, nil
}
