package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"safe-ride/internal/model"
)

var (
	ErrDriverUserNotFound      = errors.New("driver user not found")
	ErrDriverAlreadyRegistered = errors.New("user is already registered as a driver")
	ErrPlateAlreadyRegistered  = errors.New("vehicle plate number is already registered")
)

type DriverRepository struct {
	db *sql.DB
}

func NewDriverRepository(db *sql.DB) *DriverRepository {
	return &DriverRepository{db: db}
}

func (r *DriverRepository) CreateDriver(ctx context.Context, driver *model.Driver) error {
	const query = `
		INSERT INTO drivers (user_id, vehicle_type, plate_number)
		VALUES ($1, $2, $3)
		RETURNING driver_id, status
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		driver.UserID,
		driver.VehicleType,
		driver.PlateNumber,
	).Scan(&driver.DriverID, &driver.Status)
	if err == nil {
		return nil
	}

	var postgresErr *pq.Error
	if errors.As(err, &postgresErr) {
		switch {
		case postgresErr.Code == "23503" && postgresErr.Constraint == "drivers_user_id_fkey":
			return ErrDriverUserNotFound
		case postgresErr.Code == "23505" && postgresErr.Constraint == "drivers_user_id_key":
			return ErrDriverAlreadyRegistered
		case postgresErr.Code == "23505" && postgresErr.Constraint == "drivers_plate_number_key":
			return ErrPlateAlreadyRegistered
		}
	}

	return err
}

func (r *DriverRepository) GetAvailableDrivers(
	ctx context.Context,
	latitude float64,
	longitude float64,
	radiusMeters float64,
) ([]model.AvailableDriver, error) {
	const query = `
		WITH rider_location AS (
			SELECT ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography AS point
		)
		SELECT
			d.driver_id,
			d.vehicle_type,
			d.plate_number,
			ST_Y(latest.location::geometry) AS latitude,
			ST_X(latest.location::geometry) AS longitude,
			ST_Distance(latest.location, rider_location.point) AS distance_meters
		FROM drivers AS d
		JOIN LATERAL (
			SELECT location
			FROM driver_locations
			WHERE driver_id = d.driver_id
			ORDER BY timestamp DESC, location_id DESC
			LIMIT 1
		) AS latest ON TRUE
		CROSS JOIN rider_location
		WHERE d.status = 'AVAILABLE'
			AND ST_DWithin(latest.location, rider_location.point, $3)
		ORDER BY distance_meters ASC, d.driver_id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, longitude, latitude, radiusMeters)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	drivers := make([]model.AvailableDriver, 0)
	for rows.Next() {
		var driver model.AvailableDriver
		if err := rows.Scan(
			&driver.DriverID,
			&driver.VehicleType,
			&driver.PlateNumber,
			&driver.Location.Latitude,
			&driver.Location.Longitude,
			&driver.DistanceM,
		); err != nil {
			return nil, err
		}
		drivers = append(drivers, driver)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return drivers, nil
}
