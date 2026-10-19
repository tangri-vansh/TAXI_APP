package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"safe-ride/internal/model"
)

var ErrLocationDriverNotFound = errors.New("driver not found for location update")

type DriverLocationRepository struct {
	db *sql.DB
}

func NewDriverLocationRepository(db *sql.DB) *DriverLocationRepository {
	return &DriverLocationRepository{db: db}
}

func (r *DriverLocationRepository) CreateLocation(
	ctx context.Context,
	location *model.DriverLocation,
) error {
	const query = `
		INSERT INTO driver_locations (driver_id, location)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography)
		RETURNING location_id
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		location.DriverID,
		location.Longitude,
		location.Latitude,
	).Scan(&location.LocationID)
	if err == nil {
		return nil
	}

	var postgresErr *pq.Error
	if errors.As(err, &postgresErr) &&
		postgresErr.Code == "23503" &&
		postgresErr.Constraint == "driver_locations_driver_id_fkey" {
		return ErrLocationDriverNotFound
	}
	return err
}
