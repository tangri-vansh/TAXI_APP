package service

import (
	"context"
	"errors"
	"math"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
)

var (
	ErrInvalidLocation        = errors.New("invalid location")
	ErrLocationDriverNotFound = repository.ErrLocationDriverNotFound
)

type DriverLocationService struct {
	locationRepository *repository.DriverLocationRepository
}

func NewDriverLocationService(locationRepository *repository.DriverLocationRepository) *DriverLocationService {
	return &DriverLocationService{locationRepository: locationRepository}
}

func (s *DriverLocationService) CreateLocation(
	ctx context.Context,
	driverID int64,
	latitude float64,
	longitude float64,
) error {
	if driverID <= 0 || math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) ||
		latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return ErrInvalidLocation
	}

	location := &model.DriverLocation{
		DriverID:  driverID,
		Latitude:  latitude,
		Longitude: longitude,
	}
	return s.locationRepository.CreateLocation(ctx, location)
}
