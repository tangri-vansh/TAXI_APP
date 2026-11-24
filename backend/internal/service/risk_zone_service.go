package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
)

var (
	ErrInvalidRiskZoneBuildOptions = errors.New("invalid risk-zone build options")
	ErrInvalidEdgeMidpoints        = errors.New("invalid edge midpoints")
	ErrInvalidSafeModeEdgeCost     = errors.New("invalid Safe Mode edge cost inputs")
	ErrRiskDataUnavailable         = errors.New("risk data is unavailable for an edge midpoint")
	ErrRiskZonesNotBuilt           = repository.ErrRiskZonesNotBuilt
	ErrRiskZoneGridInconsistent    = repository.ErrRiskZoneGridInconsistent
)

type RiskZoneService struct {
	riskZoneRepository *repository.RiskZoneRepository
}

func NewRiskZoneService(riskZoneRepository *repository.RiskZoneRepository) *RiskZoneService {
	return &RiskZoneService{riskZoneRepository: riskZoneRepository}
}

func (s *RiskZoneService) Build(
	ctx context.Context,
	gridSizeM int,
	halfLifeDays int,
	sourceName string,
	asOf time.Time,
) (model.RiskZoneBuildSummary, error) {
	sourceName = strings.TrimSpace(sourceName)
	if gridSizeM < 100 || gridSizeM > 5000 || halfLifeDays < 1 || halfLifeDays > 3650 || sourceName == "" {
		return model.RiskZoneBuildSummary{}, ErrInvalidRiskZoneBuildOptions
	}
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}

	return s.riskZoneRepository.Rebuild(ctx, model.RiskZoneBuildOptions{
		SourceName:          sourceName,
		GridSizeM:           gridSizeM,
		RecencyHalfLifeDays: halfLifeDays,
		AsOf:                asOf.UTC(),
	})
}

func (s *RiskZoneService) LookupEdgeMidpoints(
	ctx context.Context,
	sourceName string,
	midpoints []model.Coordinates,
) ([]model.EdgeRisk, error) {
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" || len(midpoints) == 0 {
		return nil, ErrInvalidEdgeMidpoints
	}
	for _, point := range midpoints {
		if math.IsNaN(point.Latitude) || math.IsInf(point.Latitude, 0) ||
			math.IsNaN(point.Longitude) || math.IsInf(point.Longitude, 0) ||
			point.Latitude < -90 || point.Latitude > 90 ||
			point.Longitude < -180 || point.Longitude > 180 {
			return nil, ErrInvalidEdgeMidpoints
		}
	}
	edgeRisks, err := s.riskZoneRepository.LookupEdgeMidpoints(ctx, sourceName, midpoints)
	if errors.Is(err, ErrRiskZonesNotBuilt) {
		// No score grid means there is no evidence to assign to any edge. Keep
		// those edges explicitly unavailable so the Safe Mode planner can return
		// the ordinary route with a coverage explanation instead of failing or
		// silently treating missing data as zero risk.
		edgeRisks = make([]model.EdgeRisk, len(midpoints))
		for i, midpoint := range midpoints {
			edgeRisks[i] = model.EdgeRisk{EdgeIndex: i, Midpoint: midpoint}
		}
		return edgeRisks, nil
	}
	return edgeRisks, err
}

func (s *RiskZoneService) RoutingPolygons(ctx context.Context, sourceName string) ([]model.RiskZonePolygon, error) {
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" {
		return nil, ErrInvalidEdgeMidpoints
	}
	return s.riskZoneRepository.RoutingPolygons(ctx, sourceName)
}

func SafeModeEdgeCost(travelTimeSeconds, beta float64, edgeRisk model.EdgeRisk) (float64, error) {
	if math.IsNaN(travelTimeSeconds) || math.IsInf(travelTimeSeconds, 0) || travelTimeSeconds < 0 ||
		math.IsNaN(beta) || math.IsInf(beta, 0) || beta < 0 {
		return 0, ErrInvalidSafeModeEdgeCost
	}
	if !edgeRisk.RiskAvailable || edgeRisk.NormalizedRisk == nil {
		return 0, ErrRiskDataUnavailable
	}
	if math.IsNaN(*edgeRisk.NormalizedRisk) || math.IsInf(*edgeRisk.NormalizedRisk, 0) ||
		*edgeRisk.NormalizedRisk < 0 || *edgeRisk.NormalizedRisk > 1 {
		return 0, ErrInvalidSafeModeEdgeCost
	}
	cost := travelTimeSeconds + beta*(*edgeRisk.NormalizedRisk)
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return 0, ErrInvalidSafeModeEdgeCost
	}
	return cost, nil
}
