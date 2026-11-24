package service

import (
	"context"
	"errors"
	"fmt"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
	"safe-ride/internal/routing"
)

var ErrRouteNotYetAvailable = errors.New("route is not available")

// RideRouteService calculates the road route for a ride, applying the stored
// Safe Mode preference. Routes are currently calculated on demand; persistence
// will be added with the ROUTE table implementation.
type RideRouteService struct {
	rides      *repository.RideRepository
	routes     *repository.RouteRepository
	valhalla   *routing.ValhallaClient
	safeEngine *routing.SafeModeValhallaEngine
}

type RideRoute struct {
	RouteID            int64          `json:"route_id,omitempty"`
	RideID             int64          `json:"ride_id"`
	Mode               string         `json:"mode"`
	DistanceKM         float64        `json:"distance_km"`
	DurationSeconds    float64        `json:"duration_seconds"`
	Legs               []RideRouteLeg `json:"legs"`
	SafeModeUsed       bool           `json:"safe_mode_used"`
	Fallback           bool           `json:"fallback"`
	FallbackReason     string         `json:"fallback_reason,omitempty"`
	RiskExposure       *float64       `json:"risk_exposure,omitempty"`
	NormalDurationSecs *float64       `json:"normal_duration_seconds,omitempty"`
	SelectedBeta       *float64       `json:"selected_beta,omitempty"`
}

type RideRouteLeg struct {
	DistanceKM      float64 `json:"distance_km"`
	DurationSeconds float64 `json:"duration_seconds"`
	Shape           string  `json:"shape"`
}

// NewRideRouteService joins ride lookup with the existing routing adapters.
func NewRideRouteService(
	rides *repository.RideRepository,
	routes *repository.RouteRepository,
	valhalla *routing.ValhallaClient,
	safeEngine *routing.SafeModeValhallaEngine,
) *RideRouteService {
	return &RideRouteService{rides: rides, routes: routes, valhalla: valhalla, safeEngine: safeEngine}
}

func (s *RideRouteService) GetRoute(ctx context.Context, rideID int64) (*RideRoute, error) {
	if rideID <= 0 || s == nil || s.rides == nil || s.routes == nil || s.valhalla == nil {
		return nil, ErrRouteNotYetAvailable
	}
	ride, err := s.rides.GetRide(ctx, rideID)
	if err != nil {
		return nil, err
	}
	locations := []model.Coordinates{ride.Pickup, ride.Destination}
	if ride.Status == "ONGOING" {
		routingContext, err := s.rides.GetRideStopRoutingContext(ctx, rideID, ride.RiderID)
		if err != nil {
			return nil, err
		}
		locations = make([]model.Coordinates, 0, len(routingContext.PendingStops)+2)
		locations = append(locations, routingContext.CurrentLocation)
		for _, stop := range routingContext.PendingStops {
			locations = append(locations, stop.Location)
		}
		locations = append(locations, ride.Destination)
	}
	if ride.Status != "ONGOING" {
		if saved, err := s.routes.GetLatest(ctx, rideID); err == nil {
			return routeResponseFromSnapshot(saved), nil
		} else if !errors.Is(err, repository.ErrRouteSnapshotNotFound) {
			return nil, err
		}
	}
	// Ongoing rides use the latest driver location and pending stops, so refresh
	// their snapshot instead of returning a route that starts from an old point.
	return s.CalculateAndPersistForRide(ctx, *ride, locations)
}

func (s *RideRouteService) CalculateAndPersistForRide(
	ctx context.Context,
	ride model.Ride,
	locations []model.Coordinates,
) (*RideRoute, error) {
	if s == nil || s.routes == nil {
		return nil, ErrRouteNotYetAvailable
	}
	calculated, err := s.CalculateForRide(ctx, ride, locations)
	if err != nil {
		return nil, err
	}
	snapshot := routeSnapshotFromResponse(calculated)
	if err := s.routes.Create(ctx, snapshot); err != nil {
		return nil, err
	}
	calculated.RouteID = snapshot.RouteID
	return calculated, nil
}

func RouteSnapshotFromResponse(route *RideRoute) *model.RouteSnapshot {
	return routeSnapshotFromResponse(route)
}

func routeSnapshotFromResponse(route *RideRoute) *model.RouteSnapshot {
	snapshot := &model.RouteSnapshot{
		RideID: route.RideID, Mode: route.Mode, DistanceKM: route.DistanceKM,
		DurationSeconds: route.DurationSeconds, SafeModeUsed: route.SafeModeUsed,
		Fallback: route.Fallback, FallbackReason: route.FallbackReason,
		RiskExposure: route.RiskExposure, NormalDurationSecs: route.NormalDurationSecs,
		SelectedBeta: route.SelectedBeta,
		Geometry:     make([]model.RouteLeg, 0, len(route.Legs)),
	}
	for _, leg := range route.Legs {
		snapshot.Geometry = append(snapshot.Geometry, model.RouteLeg{
			DistanceKM: leg.DistanceKM, DurationSeconds: leg.DurationSeconds, Shape: leg.Shape,
		})
	}
	return snapshot
}

func routeResponseFromSnapshot(snapshot *model.RouteSnapshot) *RideRoute {
	response := &RideRoute{
		RouteID: snapshot.RouteID, RideID: snapshot.RideID, Mode: snapshot.Mode,
		DistanceKM: snapshot.DistanceKM, DurationSeconds: snapshot.DurationSeconds,
		SafeModeUsed: snapshot.SafeModeUsed, Fallback: snapshot.Fallback,
		FallbackReason: snapshot.FallbackReason, RiskExposure: snapshot.RiskExposure,
		NormalDurationSecs: snapshot.NormalDurationSecs, SelectedBeta: snapshot.SelectedBeta,
		Legs: make([]RideRouteLeg, 0, len(snapshot.Geometry)),
	}
	for _, leg := range snapshot.Geometry {
		response.Legs = append(response.Legs, RideRouteLeg{
			DistanceKM: leg.DistanceKM, DurationSeconds: leg.DurationSeconds, Shape: leg.Shape,
		})
	}
	return response
}

// CalculateForRide calculates a route through the locations in order. It is
// also used to compare the current route with a proposed route containing a
// new stop.
func (s *RideRouteService) CalculateForRide(
	ctx context.Context,
	ride model.Ride,
	locations []model.Coordinates,
) (*RideRoute, error) {
	if s == nil || s.valhalla == nil {
		return nil, ErrRouteNotYetAvailable
	}
	routeLocations := make([]routing.Coordinate, 0, len(locations))
	for _, location := range locations {
		routeLocations = append(routeLocations, routing.Coordinate{
			Latitude: location.Latitude, Longitude: location.Longitude,
		})
	}

	if !ride.SafeMode {
		route, err := s.valhalla.Route(ctx, routeLocations)
		if err != nil {
			if errors.Is(err, routing.ErrNoRouteFound) {
				return nil, ErrRouteNotYetAvailable
			}
			return nil, fmt.Errorf("calculate normal ride route: %w", err)
		}
		return toRideRoute(ride.RideID, "NORMAL", route), nil
	}
	if s.safeEngine == nil {
		return nil, fmt.Errorf("Safe Mode route engine is not configured")
	}

	// Beta is a per-edge penalty in seconds for a normalized risk score in
	// [0,1]. The descending schedule favors risk first, then progressively
	// relaxes toward travel time. Keep the number of attempts within the
	// planner's bounded-retry limit.
	result, err := s.safeEngine.Plan(ctx, routeLocations, []float64{120, 60, 30, 15, 7.5})
	if err != nil {
		if errors.Is(err, routing.ErrNoRouteFound) {
			return nil, ErrRouteNotYetAvailable
		}
		return nil, fmt.Errorf("plan Safe Mode ride route: %w", err)
	}

	mode := "NORMAL_FALLBACK"
	if result.UsedSafeMode {
		mode = "SAFE_MODE"
	}
	route := toRideRoute(ride.RideID, mode, result.Route)
	route.SafeModeUsed = result.UsedSafeMode
	route.Fallback = result.Fallback
	route.FallbackReason = result.FallbackReason
	riskExposure := result.RiskExposure
	normalDuration := result.NormalDurationSec
	route.RiskExposure = &riskExposure
	route.NormalDurationSecs = &normalDuration
	route.SelectedBeta = result.SelectedBeta
	return route, nil
}

func toRideRoute(rideID int64, mode string, route *routing.Route) *RideRoute {
	result := &RideRoute{
		RideID:          rideID,
		Mode:            mode,
		DistanceKM:      route.DistanceKM,
		DurationSeconds: route.DurationSec,
		Legs:            make([]RideRouteLeg, 0, len(route.Legs)),
	}
	for _, leg := range route.Legs {
		result.Legs = append(result.Legs, RideRouteLeg{
			DistanceKM:      leg.DistanceKM,
			DurationSeconds: leg.DurationSec,
			Shape:           leg.Shape,
		})
	}
	return result
}
