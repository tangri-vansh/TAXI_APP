package routing

import (
	"context"
	"errors"
	"testing"
)

type safeModeResponse struct {
	evaluation *RouteEvaluation
	err        error
}

type safeModeStub struct {
	responses []safeModeResponse
	calls     int
}

func (s *safeModeStub) RouteForCost(context.Context, []Coordinate, *float64) (*RouteEvaluation, error) {
	response := s.responses[s.calls]
	s.calls++
	return response.evaluation, response.err
}

func TestPlanSafeModeAcceptsLowerRiskRouteWithinDetour(t *testing.T) {
	engine := &safeModeStub{responses: []safeModeResponse{
		{evaluation: evaluation(600, 10, true)},
		{evaluation: evaluation(700, 7, true)},
	}}
	result, err := PlanSafeMode(context.Background(), engine, []Coordinate{{}, {}}, SafeModeConfig{BetaValues: []float64{20}})
	if err != nil {
		t.Fatalf("PlanSafeMode() error = %v", err)
	}
	if !result.UsedSafeMode || result.Fallback || result.Route.DurationSec != 700 || result.RiskExposure != 7 {
		t.Fatalf("unexpected Safe Mode result: %+v", result)
	}
}

func TestPlanSafeModeFallsBackWhenDetourExceedsTwentyPercent(t *testing.T) {
	engine := &safeModeStub{responses: []safeModeResponse{
		{evaluation: evaluation(600, 10, true)},
		{evaluation: evaluation(721, 2, true)},
	}}
	result, err := PlanSafeMode(context.Background(), engine, []Coordinate{{}, {}}, SafeModeConfig{BetaValues: []float64{20}})
	if err != nil {
		t.Fatalf("PlanSafeMode() error = %v", err)
	}
	if result.UsedSafeMode || !result.Fallback || result.Route.DurationSec != 600 {
		t.Fatalf("expected normal route fallback, got %+v", result)
	}
}

func TestPlanSafeModeFallsBackWhenBaselineRiskCoverageIsIncomplete(t *testing.T) {
	engine := &safeModeStub{responses: []safeModeResponse{
		{evaluation: evaluation(600, 0, false)},
	}}
	result, err := PlanSafeMode(context.Background(), engine, []Coordinate{{}, {}}, SafeModeConfig{BetaValues: []float64{20}})
	if err != nil {
		t.Fatalf("PlanSafeMode() error = %v", err)
	}
	if result.UsedSafeMode || !result.Fallback || engine.calls != 1 {
		t.Fatalf("expected immediate normal fallback for incomplete coverage, got %+v (calls %d)", result, engine.calls)
	}
}

func TestPlanSafeModeContinuesAfterCandidateHasNoRoute(t *testing.T) {
	engine := &safeModeStub{responses: []safeModeResponse{
		{evaluation: evaluation(600, 10, true)},
		{err: ErrNoRouteFound},
		{evaluation: evaluation(650, 8, true)},
	}}
	result, err := PlanSafeMode(context.Background(), engine, []Coordinate{{}, {}}, SafeModeConfig{BetaValues: []float64{20, 10}})
	if err != nil {
		t.Fatalf("PlanSafeMode() error = %v", err)
	}
	if !result.UsedSafeMode || result.Route.DurationSec != 650 || engine.calls != 3 {
		t.Fatalf("expected second beta candidate to be selected, got %+v (calls %d)", result, engine.calls)
	}
}

func TestPlanSafeModeRejectsInvalidBetaSchedule(t *testing.T) {
	engine := &safeModeStub{}
	_, err := PlanSafeMode(context.Background(), engine, []Coordinate{{}, {}}, SafeModeConfig{BetaValues: []float64{10, 20}})
	if !errors.Is(err, ErrInvalidSafeModeConfig) {
		t.Fatalf("expected ErrInvalidSafeModeConfig, got %v", err)
	}
}

func evaluation(duration, exposure float64, coverage bool) *RouteEvaluation {
	return &RouteEvaluation{
		Route:                &Route{DurationSec: duration, DistanceKM: 5},
		RiskExposure:         exposure,
		RiskCoverageComplete: coverage,
	}
}
