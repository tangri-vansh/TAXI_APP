package service

import (
	"context"
	"errors"
	"math"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
)

const (
	MaxStopDetourRatio       = 0.20
	MaxStopAddedDurationSecs = 10 * 60
	MinTimeBeforeDropoffSecs = 5 * 60
)

var (
	ErrRideNotOngoingCannotAddStop = repository.ErrRideNotOngoingCannotAddStop
	ErrNotAuthorizedToAddStop      = repository.ErrNotAuthorizedToAddStop
	ErrDriverLocationUnavailable   = repository.ErrDriverLocationUnavailable
	ErrRideStopsChanged            = repository.ErrRideStopsChanged
	ErrInvalidStopLocation         = errors.New("stop location is missing or invalid")
	ErrStopDetourTooLarge          = errors.New("stop exceeds the allowed route detour")
	ErrStopAddWindowExpired        = errors.New("too little time remains before dropoff")
)

type RideStopService struct {
	repository *repository.RideRepository
	routes     *RideRouteService
}

func NewRideStopService(repo *repository.RideRepository, routes *RideRouteService) *RideStopService {
	return &RideStopService{repository: repo, routes: routes}
}

func (s *RideStopService) AddStop(
	ctx context.Context,
	rideID, riderID int64,
	location model.Coordinates,
) (*model.RideStop, error) {
	if rideID <= 0 {
		return nil, repository.ErrRideNotFound
	}
	if riderID <= 0 {
		return nil, ErrNotAuthorizedToAddStop
	}
	if !validCoordinates(location) {
		return nil, ErrInvalidStopLocation
	}
	if s == nil || s.repository == nil || s.routes == nil {
		return nil, errors.New("ride stop dependencies are not configured")
	}

	routeContext, err := s.repository.GetRideStopRoutingContext(ctx, rideID, riderID)
	if err != nil {
		return nil, err
	}
	if sameCoordinate(location, routeContext.CurrentLocation) || sameCoordinate(location, routeContext.Ride.Destination) {
		return nil, ErrInvalidStopLocation
	}

	baseLocations := make([]model.Coordinates, 0, len(routeContext.PendingStops)+2)
	baseLocations = append(baseLocations, routeContext.CurrentLocation)
	for _, stop := range routeContext.PendingStops {
		baseLocations = append(baseLocations, stop.Location)
	}
	baseLocations = append(baseLocations, routeContext.Ride.Destination)
	baseline, err := s.routes.CalculateForRide(ctx, routeContext.Ride, baseLocations)
	if err != nil {
		return nil, err
	}
	if baseline.DurationSeconds <= MinTimeBeforeDropoffSecs {
		return nil, ErrStopAddWindowExpired
	}

	proposedLocations := make([]model.Coordinates, 0, len(baseLocations)+1)
	proposedLocations = append(proposedLocations, baseLocations[:len(baseLocations)-1]...)
	proposedLocations = append(proposedLocations, location, routeContext.Ride.Destination)
	proposed, err := s.routes.CalculateForRide(ctx, routeContext.Ride, proposedLocations)
	if err != nil {
		return nil, err
	}
	if stopDetourExceedsLimits(baseline.DistanceKM, baseline.DurationSeconds, proposed.DistanceKM, proposed.DurationSeconds) {
		return nil, ErrStopDetourTooLarge
	}

	return s.repository.CreateRideStop(
		ctx, rideID, riderID, len(routeContext.PendingStops), location,
		RouteSnapshotFromResponse(proposed),
	)
}

func stopDetourExceedsLimits(baseDistanceKM, baseDurationSecs, proposedDistanceKM, proposedDurationSecs float64) bool {
	return proposedDistanceKM-baseDistanceKM > baseDistanceKM*MaxStopDetourRatio ||
		proposedDurationSecs-baseDurationSecs > MaxStopAddedDurationSecs
}

func sameCoordinate(a, b model.Coordinates) bool {
	const tolerance = 1e-7
	return math.Abs(a.Latitude-b.Latitude) < tolerance && math.Abs(a.Longitude-b.Longitude) < tolerance
}
