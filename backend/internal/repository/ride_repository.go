package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"safe-ride/internal/model"
)

var (
	ErrRiderNotFound                = errors.New("rider not found")
	ErrRiderHasActiveRide           = errors.New("rider already has an active ride")
	ErrRideNotFound                 = errors.New("ride not found")
	ErrRideAlreadyAccepted          = errors.New("ride already accepted")
	ErrRideAlreadyCompleted         = errors.New("ride already completed")
	ErrRideCancelled                = errors.New("ride cancelled")
	ErrDriverNotFound               = errors.New("driver not found")
	ErrDriverNotAvailable           = errors.New("driver not available")
	ErrRideNotAcceptedCannotStart   = errors.New("ride must be accepted before starting")
	ErrRideAlreadyOngoing           = errors.New("ride already ongoing")
	ErrRideNotOngoingCannotComplete = errors.New("ride must be ongoing before completing")
	ErrDriverNotAssigned            = errors.New("driver is not assigned to ride")
	ErrRideOngoingCannotCancel      = errors.New("ongoing ride cannot be cancelled")
	ErrRideAlreadyCancelled         = errors.New("ride already cancelled")
	ErrNotAuthorizedToCancel        = errors.New("caller is not authorized to cancel ride")
	ErrRideNotOngoingCannotAddStop  = errors.New("ride is not ongoing")
	ErrNotAuthorizedToAddStop       = errors.New("caller is not authorized to add a stop")
	ErrDriverLocationUnavailable    = errors.New("current driver location is unavailable")
	ErrRideStopsChanged             = errors.New("ride stops changed while route was being checked")
)

type RideStopRoutingContext struct {
	Ride            model.Ride
	CurrentLocation model.Coordinates
	PendingStops    []model.RideStop
}

type RideRepository struct {
	db *sql.DB
}

func NewRideRepository(db *sql.DB) *RideRepository {
	return &RideRepository{db: db}
}

func (r *RideRepository) CreateRide(ctx context.Context, ride *model.Ride) error {
	const query = `
		INSERT INTO rides (rider_id, pickup, destination, safe_mode, status)
		VALUES (
			$1,
			ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography,
			ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
			$6,
			'REQUESTED'
		)
		RETURNING ride_id, status
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		ride.RiderID,
		ride.Pickup.Longitude,
		ride.Pickup.Latitude,
		ride.Destination.Longitude,
		ride.Destination.Latitude,
		ride.SafeMode,
	).Scan(&ride.RideID, &ride.Status)
	if err == nil {
		return nil
	}

	var postgresErr *pq.Error
	if errors.As(err, &postgresErr) {
		switch {
		case postgresErr.Code == "23503" && postgresErr.Constraint == "rides_rider_id_fkey":
			return ErrRiderNotFound
		case postgresErr.Code == "23505" && postgresErr.Constraint == "rides_one_active_per_rider_unique_idx":
			return ErrRiderHasActiveRide
		}
	}
	return err
}

func (r *RideRepository) GetRide(ctx context.Context, rideID int64) (*model.Ride, error) {
	const query = `
		SELECT
			ride_id,
			rider_id,
			driver_id,
			ST_Y(pickup::geometry),
			ST_X(pickup::geometry),
			ST_Y(destination::geometry),
			ST_X(destination::geometry),
			status,
			safe_mode
		FROM rides
		WHERE ride_id = $1
	`

	var ride model.Ride
	var driverID sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, rideID).Scan(
		&ride.RideID,
		&ride.RiderID,
		&driverID,
		&ride.Pickup.Latitude,
		&ride.Pickup.Longitude,
		&ride.Destination.Latitude,
		&ride.Destination.Longitude,
		&ride.Status,
		&ride.SafeMode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRideNotFound
	}
	if err != nil {
		return nil, err
	}
	if driverID.Valid {
		id := driverID.Int64
		ride.DriverID = &id
	}
	return &ride, nil
}

// GetRideStopRoutingContext reads the ongoing ride, the latest driver position,
// and its pending stops from one consistent database snapshot.
func (r *RideRepository) GetRideStopRoutingContext(
	ctx context.Context,
	rideID, riderID int64,
) (*RideStopRoutingContext, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var result RideStopRoutingContext
	var driverID sql.NullInt64
	var latitude, longitude sql.NullFloat64
	err = tx.QueryRowContext(ctx, `
		SELECT ride.ride_id, ride.rider_id, ride.driver_id,
			ST_Y(ride.pickup::geometry), ST_X(ride.pickup::geometry),
			ST_Y(ride.destination::geometry), ST_X(ride.destination::geometry),
			ride.status, ride.safe_mode,
			ST_Y(current_location.location::geometry), ST_X(current_location.location::geometry)
		FROM rides AS ride
		LEFT JOIN LATERAL (
			SELECT location
			FROM driver_locations
			WHERE driver_id = ride.driver_id
			ORDER BY timestamp DESC, location_id DESC
			LIMIT 1
		) AS current_location ON TRUE
		WHERE ride.ride_id = $1`, rideID).Scan(
		&result.Ride.RideID, &result.Ride.RiderID, &driverID,
		&result.Ride.Pickup.Latitude, &result.Ride.Pickup.Longitude,
		&result.Ride.Destination.Latitude, &result.Ride.Destination.Longitude,
		&result.Ride.Status, &result.Ride.SafeMode, &latitude, &longitude,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRideNotFound
	}
	if err != nil {
		return nil, err
	}
	if result.Ride.Status != "ONGOING" {
		return nil, ErrRideNotOngoingCannotAddStop
	}
	if result.Ride.RiderID != riderID {
		return nil, ErrNotAuthorizedToAddStop
	}
	if !driverID.Valid || !latitude.Valid || !longitude.Valid {
		return nil, ErrDriverLocationUnavailable
	}
	result.Ride.DriverID = &driverID.Int64
	result.CurrentLocation = model.Coordinates{Latitude: latitude.Float64, Longitude: longitude.Float64}

	rows, err := tx.QueryContext(ctx, `
		SELECT stop_id, ride_id, ST_Y(location::geometry), ST_X(location::geometry), sequence_order, status
		FROM ride_stops
		WHERE ride_id = $1 AND status = 'PENDING'
		ORDER BY sequence_order`, rideID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.PendingStops = make([]model.RideStop, 0)
	for rows.Next() {
		var stop model.RideStop
		if err := rows.Scan(&stop.StopID, &stop.RideID, &stop.Location.Latitude, &stop.Location.Longitude, &stop.SequenceOrder, &stop.Status); err != nil {
			return nil, err
		}
		result.PendingStops = append(result.PendingStops, stop)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateRideStop locks the ride and assigns the next sequence number. The
// expected pending-stop count prevents committing a detour check based on a
// stale route if another stop was added while Valhalla was routing.
func (r *RideRepository) CreateRideStop(
	ctx context.Context,
	rideID, riderID int64,
	expectedPendingCount int,
	location model.Coordinates,
	route *model.RouteSnapshot,
) (*model.RideStop, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var ownerID int64
	var status string
	err = tx.QueryRowContext(ctx, `SELECT rider_id, status FROM rides WHERE ride_id = $1 FOR UPDATE`, rideID).Scan(&ownerID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRideNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "ONGOING" {
		return nil, ErrRideNotOngoingCannotAddStop
	}
	if ownerID != riderID {
		return nil, ErrNotAuthorizedToAddStop
	}

	var actualCount int
	var nextOrder int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FILTER (WHERE status = 'PENDING'), COALESCE(MAX(sequence_order), 0) + 1
		FROM ride_stops WHERE ride_id = $1`, rideID).Scan(&actualCount, &nextOrder); err != nil {
		return nil, err
	}
	if actualCount != expectedPendingCount {
		return nil, ErrRideStopsChanged
	}
	var stop model.RideStop
	stop.RideID = rideID
	stop.Location = location
	stop.SequenceOrder = nextOrder
	stop.Status = "PENDING"
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ride_stops (ride_id, location, sequence_order, status)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, 'PENDING')
		RETURNING stop_id`, rideID, location.Longitude, location.Latitude, nextOrder).Scan(&stop.StopID)
	if err != nil {
		return nil, err
	}
	if route == nil || route.RideID != rideID {
		return nil, errors.New("route snapshot does not match ride")
	}
	if err := insertRoute(ctx, tx, route); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &stop, nil
}

func (r *RideRepository) AcceptRide(ctx context.Context, rideID, driverID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(
		ctx,
		`SELECT status FROM rides WHERE ride_id = $1 FOR UPDATE`,
		rideID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRideNotFound
	}
	if err != nil {
		return err
	}
	switch status {
	case "REQUESTED":
		// Continue after validating and locking the requested driver.
	case "ACCEPTED", "ONGOING":
		return ErrRideAlreadyAccepted
	case "COMPLETED":
		return ErrRideAlreadyCompleted
	case "CANCELLED":
		return ErrRideCancelled
	}

	var driverStatus string
	err = tx.QueryRowContext(
		ctx,
		`SELECT status FROM drivers WHERE driver_id = $1 FOR UPDATE`,
		driverID,
	).Scan(&driverStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDriverNotFound
	}
	if err != nil {
		return err
	}
	if driverStatus != "AVAILABLE" {
		return ErrDriverNotAvailable
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE drivers SET status = 'BUSY' WHERE driver_id = $1`,
		driverID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE rides SET driver_id = $2, status = 'ACCEPTED' WHERE ride_id = $1`,
		rideID,
		driverID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *RideRepository) StartRide(ctx context.Context, rideID, driverID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	status, assignedDriverID, err := lockRideState(ctx, tx, rideID)
	if err != nil {
		return err
	}
	switch status {
	case "REQUESTED":
		return ErrRideNotAcceptedCannotStart
	case "ONGOING":
		return ErrRideAlreadyOngoing
	case "COMPLETED":
		return ErrRideAlreadyCompleted
	case "CANCELLED":
		return ErrRideCancelled
	}
	if !assignedDriverID.Valid || assignedDriverID.Int64 != driverID {
		return ErrDriverNotAssigned
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rides SET status = 'ONGOING' WHERE ride_id = $1`, rideID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *RideRepository) CompleteRide(ctx context.Context, rideID, driverID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	status, assignedDriverID, err := lockRideState(ctx, tx, rideID)
	if err != nil {
		return err
	}
	switch status {
	case "REQUESTED", "ACCEPTED":
		return ErrRideNotOngoingCannotComplete
	case "COMPLETED":
		return ErrRideAlreadyCompleted
	case "CANCELLED":
		return ErrRideCancelled
	}
	if !assignedDriverID.Valid || assignedDriverID.Int64 != driverID {
		return ErrDriverNotAssigned
	}

	var lockedDriverID int64
	err = tx.QueryRowContext(
		ctx,
		`SELECT driver_id FROM drivers WHERE driver_id = $1 FOR UPDATE`,
		driverID,
	).Scan(&lockedDriverID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDriverNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rides SET status = 'COMPLETED' WHERE ride_id = $1`, rideID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE drivers SET status = 'AVAILABLE' WHERE driver_id = $1`, driverID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *RideRepository) CancelRide(ctx context.Context, rideID, riderID, driverID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	var storedRiderID int64
	var assignedDriverID sql.NullInt64
	err = tx.QueryRowContext(
		ctx,
		`SELECT status, rider_id, driver_id FROM rides WHERE ride_id = $1 FOR UPDATE`,
		rideID,
	).Scan(&status, &storedRiderID, &assignedDriverID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRideNotFound
	}
	if err != nil {
		return err
	}

	switch status {
	case "REQUESTED":
		if riderID == 0 || riderID != storedRiderID || driverID != 0 {
			return ErrNotAuthorizedToCancel
		}
	case "ACCEPTED":
		riderIsCaller := riderID != 0 && riderID == storedRiderID && driverID == 0
		driverIsCaller := assignedDriverID.Valid && driverID != 0 &&
			driverID == assignedDriverID.Int64 && riderID == 0
		if !riderIsCaller && !driverIsCaller {
			return ErrNotAuthorizedToCancel
		}
	case "ONGOING":
		return ErrRideOngoingCannotCancel
	case "COMPLETED":
		return ErrRideAlreadyCompleted
	case "CANCELLED":
		return ErrRideAlreadyCancelled
	}

	if assignedDriverID.Valid {
		var lockedDriverID int64
		err = tx.QueryRowContext(
			ctx,
			`SELECT driver_id FROM drivers WHERE driver_id = $1 FOR UPDATE`,
			assignedDriverID.Int64,
		).Scan(&lockedDriverID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE drivers SET status = 'AVAILABLE' WHERE driver_id = $1`,
			assignedDriverID.Int64,
		); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE rides SET status = 'CANCELLED' WHERE ride_id = $1`, rideID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockRideState(ctx context.Context, tx *sql.Tx, rideID int64) (string, sql.NullInt64, error) {
	var status string
	var driverID sql.NullInt64
	err := tx.QueryRowContext(
		ctx,
		`SELECT status, driver_id FROM rides WHERE ride_id = $1 FOR UPDATE`,
		rideID,
	).Scan(&status, &driverID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", sql.NullInt64{}, ErrRideNotFound
	}
	if err != nil {
		return "", sql.NullInt64{}, err
	}
	return status, driverID, nil
}
