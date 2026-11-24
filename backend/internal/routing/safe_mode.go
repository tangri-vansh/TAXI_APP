package routing

import (
	"context"
	"errors"
	"fmt"
	"math"
)

const (
	// SafeModeMaxBetaAttempts bounds route searches so a request cannot trigger
	// an unbounded number of A* runs.
	SafeModeMaxBetaAttempts   = 5
	MaxSafeModeTravelIncrease = 0.20
)

var (
	ErrInvalidSafeModeConfig  = errors.New("invalid Safe Mode routing configuration")
	ErrInvalidRouteEvaluation = errors.New("invalid route evaluation")
)

// RouteEvaluation is a route and the risk summary for its actual graph edges.
// RiskCoverageComplete must be false if any edge midpoint has no zone score;
// missing risk data must never be treated as zero risk.
type RouteEvaluation struct {
	Route                *Route
	RiskExposure         float64
	RiskCoverageComplete bool
}

// SafeModeEngine is the contract the custom Valhalla costing integration must
// implement. A nil beta requests the normal time-optimized baseline. A non-nil
// beta must be applied to edge costs during A* as Time(edge)+beta*Risk(edge).
// It must also return risk exposure calculated from the selected route's edges.
type SafeModeEngine interface {
	RouteForCost(ctx context.Context, locations []Coordinate, beta *float64) (*RouteEvaluation, error)
}

type SafeModeConfig struct {
	// BetaValues are tried in descending order. Each subsequent value should be
	// lower so retries gradually favor travel time over risk.
	BetaValues []float64
}

type SafeModeResult struct {
	Route             *Route
	RiskExposure      float64
	UsedSafeMode      bool
	Fallback          bool
	FallbackReason    string
	NormalDurationSec float64
	SelectedBeta      *float64
}

// PlanSafeMode computes the normal baseline, tries a bounded descending beta
// schedule, enforces the design's 20% ETA cap, and falls back to the normal
// route if no lower-risk candidate qualifies.
func PlanSafeMode(
	ctx context.Context,
	engine SafeModeEngine,
	locations []Coordinate,
	config SafeModeConfig,
) (*SafeModeResult, error) {
	if engine == nil || len(locations) < 2 || len(config.BetaValues) == 0 || len(config.BetaValues) > SafeModeMaxBetaAttempts {
		return nil, ErrInvalidSafeModeConfig
	}
	for i, beta := range config.BetaValues {
		if math.IsNaN(beta) || math.IsInf(beta, 0) || beta <= 0 || (i > 0 && beta >= config.BetaValues[i-1]) {
			return nil, ErrInvalidSafeModeConfig
		}
	}

	normal, err := engine.RouteForCost(ctx, locations, nil)
	if err != nil {
		return nil, fmt.Errorf("calculate normal route baseline: %w", err)
	}
	if err := validateRouteEvaluation(normal); err != nil {
		return nil, fmt.Errorf("validate normal route baseline: %w", err)
	}
	normalResult := &SafeModeResult{
		Route:             normal.Route,
		RiskExposure:      normal.RiskExposure,
		Fallback:          true,
		NormalDurationSec: normal.Route.DurationSec,
	}
	if !normal.RiskCoverageComplete {
		normalResult.FallbackReason = "Risk data coverage is incomplete for the normal route"
		return normalResult, nil
	}

	for _, betaValue := range config.BetaValues {
		beta := betaValue
		candidate, err := engine.RouteForCost(ctx, locations, &beta)
		if err != nil {
			if errors.Is(err, ErrNoRouteFound) {
				continue
			}
			return nil, fmt.Errorf("calculate Safe Mode route for beta %.6g: %w", beta, err)
		}
		if candidate == nil {
			continue
		}
		if err := validateRouteEvaluation(candidate); err != nil {
			return nil, fmt.Errorf("validate Safe Mode route for beta %.6g: %w", beta, err)
		}
		if !candidate.RiskCoverageComplete {
			continue
		}
		if candidate.Route.DurationSec > normal.Route.DurationSec*(1+MaxSafeModeTravelIncrease) {
			continue
		}
		if candidate.RiskExposure >= normal.RiskExposure {
			continue
		}
		return &SafeModeResult{
			Route:             candidate.Route,
			RiskExposure:      candidate.RiskExposure,
			UsedSafeMode:      true,
			NormalDurationSec: normal.Route.DurationSec,
			SelectedBeta:      &beta,
		}, nil
	}

	normalResult.FallbackReason = "No lower historical reported-incident risk route fit within the 20% travel-time limit"
	return normalResult, nil
}

func validateRouteEvaluation(evaluation *RouteEvaluation) error {
	if evaluation == nil || evaluation.Route == nil ||
		math.IsNaN(evaluation.RiskExposure) || math.IsInf(evaluation.RiskExposure, 0) || evaluation.RiskExposure < 0 ||
		math.IsNaN(evaluation.Route.DurationSec) || math.IsInf(evaluation.Route.DurationSec, 0) || evaluation.Route.DurationSec <= 0 ||
		math.IsNaN(evaluation.Route.DistanceKM) || math.IsInf(evaluation.Route.DistanceKM, 0) || evaluation.Route.DistanceKM < 0 {
		return ErrInvalidRouteEvaluation
	}
	return nil
}
