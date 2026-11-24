package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"safe-ride/internal/model"
)

var ErrLocationDriverNotFound = errors.New("driver not found for location update")

const StopArrivalRadiusMeters = 50

type DriverLocationRepository struct {
	db *sql.DB
}

func NewDriverLocationRepository(db *sql.DB) *DriverLocationRepository {
	return &DriverLocationRepository{db: db}
}

func (r *DriverLocationRepository) CreateLocation(
	ctx context.Context,
	location *model.DriverLocation,
) (*int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO driver_locations (driver_id, location)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography)
		RETURNING location_id
	`

	err = tx.QueryRowContext(
		ctx,
		query,
		location.DriverID,
		location.Longitude,
		location.Latitude,
	).Scan(&location.LocationID)
	if err != nil {
		var postgresErr *pq.Error
		if errors.As(err, &postgresErr) &&
			postgresErr.Code == "23503" &&
			postgresErr.Constraint == "driver_locations_driver_id_fkey" {
			return nil, ErrLocationDriverNotFound
		}
		return nil, err
	}

	// Serialize pings for the same ongoing ride so two close-together pings
	// cannot complete multiple ordered stops at once.
	var rideID int64
	err = tx.QueryRowContext(ctx, `
		SELECT ride_id
		FROM rides
		WHERE driver_id = $1 AND status = 'ONGOING'
		ORDER BY ride_id
		LIMIT 1
		FOR UPDATE`, location.DriverID).Scan(&rideID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var completedStopID *int64
	if err == nil {
		var stopID int64
		var reached bool
		err = tx.QueryRowContext(ctx, `
			SELECT stop_id,
				ST_DWithin(
					location,
					ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography,
					$4
				)
			FROM ride_stops
			WHERE ride_id = $1 AND status = 'PENDING'
			ORDER BY sequence_order
			LIMIT 1
			FOR UPDATE`, rideID, location.Longitude, location.Latitude, StopArrivalRadiusMeters).Scan(&stopID, &reached)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err == nil && reached {
			if _, err := tx.ExecContext(ctx, `UPDATE ride_stops SET status = 'COMPLETED' WHERE stop_id = $1`, stopID); err != nil {
				return nil, err
			}
			completedStopID = &stopID
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return completedStopID, nil
}
