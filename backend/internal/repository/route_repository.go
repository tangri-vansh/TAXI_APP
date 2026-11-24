package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"safe-ride/internal/model"
)

var ErrRouteSnapshotNotFound = errors.New("route snapshot not found")

type RouteRepository struct {
	db *sql.DB
}

func NewRouteRepository(db *sql.DB) *RouteRepository {
	return &RouteRepository{db: db}
}

func (r *RouteRepository) Create(ctx context.Context, route *model.RouteSnapshot) error {
	if route == nil {
		return errors.New("route snapshot is nil")
	}
	return insertRoute(ctx, r.db, route)
}

func (r *RouteRepository) GetLatest(ctx context.Context, rideID int64) (*model.RouteSnapshot, error) {
	var route model.RouteSnapshot
	var geometry []byte
	var riskExposure, normalDuration, selectedBeta sql.NullFloat64
	var fallbackReason sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT route_id, ride_id, geometry, distance_km, eta_seconds, risk_score,
			mode, safe_mode_used, fallback, fallback_reason,
			normal_duration_seconds, selected_beta, created_at
		FROM routes
		WHERE ride_id = $1
		ORDER BY created_at DESC, route_id DESC
		LIMIT 1`, rideID).Scan(
		&route.RouteID, &route.RideID, &geometry, &route.DistanceKM, &route.DurationSeconds,
		&riskExposure, &route.Mode, &route.SafeModeUsed, &route.Fallback, &fallbackReason,
		&normalDuration, &selectedBeta, &route.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRouteSnapshotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read latest route snapshot: %w", err)
	}
	if err := json.Unmarshal(geometry, &route.Geometry); err != nil {
		return nil, fmt.Errorf("decode route geometry: %w", err)
	}
	if riskExposure.Valid {
		route.RiskExposure = &riskExposure.Float64
	}
	if normalDuration.Valid {
		route.NormalDurationSecs = &normalDuration.Float64
	}
	if selectedBeta.Valid {
		route.SelectedBeta = &selectedBeta.Float64
	}
	if fallbackReason.Valid {
		route.FallbackReason = fallbackReason.String
	}
	return &route, nil
}

func insertRoute(ctx context.Context, exec interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, route *model.RouteSnapshot) error {
	geometry, err := json.Marshal(route.Geometry)
	if err != nil {
		return fmt.Errorf("encode route geometry: %w", err)
	}
	err = exec.QueryRowContext(ctx, `
		INSERT INTO routes (
			ride_id, geometry, distance_km, eta_seconds, risk_score, mode,
			safe_mode_used, fallback, fallback_reason,
			normal_duration_seconds, selected_beta
		) VALUES ($1, $2::jsonb, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING route_id, created_at`,
		route.RideID, geometry, route.DistanceKM, route.DurationSeconds, route.RiskExposure,
		route.Mode, route.SafeModeUsed, route.Fallback, nullableString(route.FallbackReason),
		route.NormalDurationSecs, route.SelectedBeta,
	).Scan(&route.RouteID, &route.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert route snapshot: %w", err)
	}
	return nil
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
