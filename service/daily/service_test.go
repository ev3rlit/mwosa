package daily

import (
	"context"
	"strings"
	"testing"

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

type fakeReadRepository struct{}

func (fakeReadRepository) QueryDailyBars(context.Context, Query) ([]dailybar.Bar, error) {
	return nil, nil
}

type fakeWriteRepository struct{}

func (fakeWriteRepository) UpsertDailyBars(context.Context, []dailybar.Bar) (WriteResult, error) {
	return WriteResult{}, nil
}

type fakeDailyBarRouter struct{}

func (fakeDailyBarRouter) RouteDailyBars(context.Context, dailybar.RouteInput) (dailybar.Fetcher, error) {
	return nil, nil
}

func (fakeDailyBarRouter) PlanDailyBars(context.Context, dailybar.RouteInput) (dailybar.RoutePlan, error) {
	return dailybar.RoutePlan{}, nil
}
