package service

import (
	"context"
	"errors"
	"math"
	"strings"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
)

const searchRadiusMeters = 2000.0

var (
	ErrInvalidDriverInput      = errors.New("invalid driver input")
	ErrDriverUserNotFound      = repository.ErrDriverUserNotFound
	ErrDriverAlreadyRegistered = repository.ErrDriverAlreadyRegistered
	ErrPlateAlreadyRegistered  = repository.ErrPlateAlreadyRegistered
)

type DriverService struct {
	driverRepository *repository.DriverRepository
}

func NewDriverService(driverRepository *repository.DriverRepository) *DriverService {
	return &DriverService{driverRepository: driverRepository}
}

func (s *DriverService) CreateDriver(
	ctx context.Context,
	userID int64,
	vehicleType string,
	plateNumber string,
) (*model.Driver, error) {
	vehicleType = strings.TrimSpace(vehicleType)
	plateNumber = strings.TrimSpace(plateNumber)
	if userID <= 0 || vehicleType == "" || plateNumber == "" {
		return nil, ErrInvalidDriverInput
	}

	driver := &model.Driver{
		UserID:      userID,
		VehicleType: vehicleType,
		PlateNumber: plateNumber,
	}
	if err := s.driverRepository.CreateDriver(ctx, driver); err != nil {
		return nil, err
	}
	return driver, nil
}

func (s *DriverService) GetAvailableDrivers(
	ctx context.Context,
	latitude float64,
	longitude float64,
) ([]model.AvailableDriver, error) {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) ||
		latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return nil, ErrInvalidDriverInput
	}
	return s.driverRepository.GetAvailableDrivers(ctx, latitude, longitude, searchRadiusMeters)
}
