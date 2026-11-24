package service

import (
	"context"
	"errors"
	"math"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
)

var (
	ErrInvalidRideInput             = errors.New("invalid ride input")
	ErrRiderNotFound                = repository.ErrRiderNotFound
	ErrRiderHasActiveRide           = repository.ErrRiderHasActiveRide
	ErrRideNotFound                 = repository.ErrRideNotFound
	ErrRideAlreadyAccepted          = repository.ErrRideAlreadyAccepted
	ErrRideAlreadyCompleted         = repository.ErrRideAlreadyCompleted
	ErrRideCancelled                = repository.ErrRideCancelled
	ErrDriverNotFound               = repository.ErrDriverNotFound
	ErrDriverNotAvailable           = repository.ErrDriverNotAvailable
	ErrRideNotAcceptedCannotStart   = repository.ErrRideNotAcceptedCannotStart
	ErrRideAlreadyOngoing           = repository.ErrRideAlreadyOngoing
	ErrRideNotOngoingCannotComplete = repository.ErrRideNotOngoingCannotComplete
	ErrDriverNotAssigned            = repository.ErrDriverNotAssigned
	ErrRideOngoingCannotCancel      = repository.ErrRideOngoingCannotCancel
	ErrRideAlreadyCancelled         = repository.ErrRideAlreadyCancelled
	ErrNotAuthorizedToCancel        = repository.ErrNotAuthorizedToCancel
)

type RideService struct {
	rideRepository *repository.RideRepository
}

func NewRideService(rideRepository *repository.RideRepository) *RideService {
	return &RideService{rideRepository: rideRepository}
}

func (s *RideService) CreateRide(
	ctx context.Context,
	riderID int64,
	pickup model.Coordinates,
	destination model.Coordinates,
	safeMode bool,
) (*model.Ride, error) {
	if riderID <= 0 {
		return nil, ErrRiderNotFound
	}
	if !validCoordinates(pickup) || !validCoordinates(destination) ||
		(pickup.Latitude == destination.Latitude && pickup.Longitude == destination.Longitude) {
		return nil, ErrInvalidRideInput
	}

	ride := &model.Ride{
		RiderID:     riderID,
		Pickup:      pickup,
		Destination: destination,
		Status:      "REQUESTED",
		SafeMode:    safeMode,
	}
	if err := s.rideRepository.CreateRide(ctx, ride); err != nil {
		return nil, err
	}
	return ride, nil
}

func (s *RideService) GetRide(ctx context.Context, rideID int64) (*model.Ride, error) {
	if rideID <= 0 {
		return nil, ErrRideNotFound
	}
	return s.rideRepository.GetRide(ctx, rideID)
}

func (s *RideService) AcceptRide(ctx context.Context, rideID, driverID int64) error {
	if rideID <= 0 {
		return ErrRideNotFound
	}
	if driverID <= 0 {
		return ErrDriverNotFound
	}
	return s.rideRepository.AcceptRide(ctx, rideID, driverID)
}

func (s *RideService) StartRide(ctx context.Context, rideID, driverID int64) error {
	if rideID <= 0 {
		return ErrRideNotFound
	}
	if driverID <= 0 {
		return ErrDriverNotAssigned
	}
	return s.rideRepository.StartRide(ctx, rideID, driverID)
}

func (s *RideService) CompleteRide(ctx context.Context, rideID, driverID int64) error {
	if rideID <= 0 {
		return ErrRideNotFound
	}
	if driverID <= 0 {
		return ErrDriverNotAssigned
	}
	return s.rideRepository.CompleteRide(ctx, rideID, driverID)
}

func (s *RideService) CancelRide(ctx context.Context, rideID, riderID, driverID int64) error {
	if rideID <= 0 {
		return ErrRideNotFound
	}
	if (riderID <= 0 && driverID <= 0) || (riderID > 0 && driverID > 0) {
		return ErrNotAuthorizedToCancel
	}
	return s.rideRepository.CancelRide(ctx, rideID, riderID, driverID)
}

func validCoordinates(coordinates model.Coordinates) bool {
	return !math.IsNaN(coordinates.Latitude) && !math.IsInf(coordinates.Latitude, 0) &&
		!math.IsNaN(coordinates.Longitude) && !math.IsInf(coordinates.Longitude, 0) &&
		coordinates.Latitude >= -90 && coordinates.Latitude <= 90 &&
		coordinates.Longitude >= -180 && coordinates.Longitude <= 180
}
