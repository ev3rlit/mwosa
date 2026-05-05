package handler

import (
	"context"

	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
)

type Strategy struct {
	service strategyservice.Service
}

func NewStrategy(service strategyservice.Service) Strategy {
	return Strategy{service: service}
}

type CreateStrategyRequest struct {
	Name         string
	Engine       strategyservice.Engine
	InputDataset string
	QueryText    string
}

type ListStrategiesRequest struct{}

type UpdateStrategyRequest struct {
	Name      string
	QueryText string
}

type DeleteStrategyRequest struct {
	Name string
}

type ScreenJQRequest struct {
	InputDataset string
	QueryText    string
}

type ScreenStrategyRequest struct {
	Name  string
	Alias string
}

type ScreenHistoryRequest struct {
	Limit int
}

type InspectStrategyRequest struct {
	Name string
}

type InspectScreenRequest struct {
	Ref string
}

func (h Strategy) Create(ctx context.Context, req CreateStrategyRequest) (StrategyDetailOutput, error) {
	detail, err := h.service.Create(ctx, strategyservice.CreateStrategyRequest{
		Name:         req.Name,
		Engine:       req.Engine,
		InputDataset: req.InputDataset,
		QueryText:    req.QueryText,
	})
	if err != nil {
		return StrategyDetailOutput{}, err
	}
	return StrategyDetailOutput{Detail: detail}, nil
}

func (h Strategy) List(ctx context.Context, _ ListStrategiesRequest) (StrategyListOutput, error) {
	details, err := h.service.List(ctx)
	if err != nil {
		return nil, err
	}
	return StrategyListOutput(details), nil
}

func (h Strategy) Update(ctx context.Context, req UpdateStrategyRequest) (StrategyDetailOutput, error) {
	detail, err := h.service.Update(ctx, strategyservice.UpdateStrategyRequest{
		Name:      req.Name,
		QueryText: req.QueryText,
	})
	if err != nil {
		return StrategyDetailOutput{}, err
	}
	return StrategyDetailOutput{Detail: detail}, nil
}

func (h Strategy) Delete(ctx context.Context, req DeleteStrategyRequest) (DeleteStrategyResult, error) {
	if err := h.service.Delete(ctx, req.Name); err != nil {
		return DeleteStrategyResult{}, err
	}
	return DeleteStrategyResult{Name: req.Name, Deleted: true}, nil
}

func (h Strategy) ScreenJQ(ctx context.Context, req ScreenJQRequest) (ScreenResultOutput, error) {
	result, err := h.service.ScreenJQ(ctx, strategyservice.ScreenJQRequest{
		InputDataset: req.InputDataset,
		QueryText:    req.QueryText,
	})
	if err != nil {
		return ScreenResultOutput{}, err
	}
	return ScreenResultOutput{Result: result}, nil
}

func (h Strategy) Screen(ctx context.Context, req ScreenStrategyRequest) (ScreenRunDetailOutput, error) {
	detail, err := h.service.Screen(ctx, strategyservice.ScreenStrategyRequest{
		Name:  req.Name,
		Alias: req.Alias,
	})
	if err != nil {
		return ScreenRunDetailOutput{}, err
	}
	return ScreenRunDetailOutput{Detail: detail}, nil
}

func (h Strategy) History(ctx context.Context, req ScreenHistoryRequest) (ScreenRunHistoryOutput, error) {
	runs, err := h.service.History(ctx, req.Limit)
	if err != nil {
		return nil, err
	}
	return ScreenRunHistoryOutput(runs), nil
}

func (h Strategy) Inspect(ctx context.Context, req InspectStrategyRequest) (StrategyDetailOutput, error) {
	detail, err := h.service.Inspect(ctx, req.Name)
	if err != nil {
		return StrategyDetailOutput{}, err
	}
	return StrategyDetailOutput{Detail: detail}, nil
}

func (h Strategy) InspectScreen(ctx context.Context, req InspectScreenRequest) (ScreenRunDetailOutput, error) {
	detail, err := h.service.InspectScreen(ctx, req.Ref)
	if err != nil {
		return ScreenRunDetailOutput{}, err
	}
	return ScreenRunDetailOutput{Detail: detail}, nil
}
