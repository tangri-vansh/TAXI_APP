package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"safe-ride/internal/model"
)

var ErrRiskZonesNotBuilt = errors.New("risk zones have not been built for this source")
var ErrRiskZoneGridInconsistent = errors.New("risk-zone source has inconsistent grid settings")

type RiskZoneRepository struct {
	db *sql.DB
}

func (r *RiskZoneRepository) RoutingPolygons(ctx context.Context, sourceName string) ([]model.RiskZonePolygon, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT normalized_risk, ST_AsGeoJSON(ST_Transform(zone_geom, 4326))
		FROM risk_zones
		WHERE source_name = $1
		ORDER BY risk_zone_id`, sourceName)
	if err != nil {
		return nil, fmt.Errorf("query risk-zone polygons for routing: %w", err)
	}
	defer rows.Close()

	type polygonJSON struct {
		Coordinates [][][]float64 `json:"coordinates"`
	}
	polygons := make([]model.RiskZonePolygon, 0)
	for rows.Next() {
		var normalizedRisk float64
		var encodedGeometry string
		if err := rows.Scan(&normalizedRisk, &encodedGeometry); err != nil {
			return nil, fmt.Errorf("scan risk-zone polygon: %w", err)
		}
		var geometry polygonJSON
		if err := json.Unmarshal([]byte(encodedGeometry), &geometry); err != nil {
			return nil, fmt.Errorf("decode risk-zone polygon: %w", err)
		}
		if len(geometry.Coordinates) == 0 || len(geometry.Coordinates[0]) < 4 {
			return nil, fmt.Errorf("risk zone has invalid polygon geometry")
		}
		polygon := model.RiskZonePolygon{
			NormalizedRisk: normalizedRisk,
			Vertices:       make([]model.Coordinates, 0, len(geometry.Coordinates[0])),
		}
		for _, coordinate := range geometry.Coordinates[0] {
			if len(coordinate) < 2 {
				return nil, fmt.Errorf("risk zone has invalid polygon coordinate")
			}
			polygon.Vertices = append(polygon.Vertices, model.Coordinates{
				Longitude: coordinate[0],
				Latitude:  coordinate[1],
			})
		}
		polygons = append(polygons, polygon)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate risk-zone polygons: %w", err)
	}
	if len(polygons) == 0 {
		return nil, ErrRiskZonesNotBuilt
	}
	return polygons, nil
}

func NewRiskZoneRepository(db *sql.DB) *RiskZoneRepository {
	return &RiskZoneRepository{db: db}
}

func (r *RiskZoneRepository) Rebuild(
	ctx context.Context,
	options model.RiskZoneBuildOptions,
) (model.RiskZoneBuildSummary, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("begin risk-zone build: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE risk_zones IN ACCESS EXCLUSIVE MODE`); err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("lock risk zones: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM risk_zones WHERE source_name = $1`, options.SourceName); err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("clear previous risk zones: %w", err)
	}

	const insertZones = `
		WITH located AS (
			SELECT
				ST_X(cell) AS center_x_m,
				ST_Y(cell) AS center_y_m,
				risk_weight,
				GREATEST(
					0.0,
					EXTRACT(EPOCH FROM ($2::timestamptz - occurred_at)) / 86400.0
				) AS age_days
			FROM (
				SELECT
					ST_SnapToGrid(ST_Transform(location::geometry, 32619), ($1::integer)::float8) AS cell,
					risk_weight,
					occurred_at
			FROM crime_incidents
			WHERE source_name = $4
			) AS projected
		), raw AS (
			SELECT
				center_x_m,
				center_y_m,
				COUNT(*) AS incident_count,
			SUM(risk_weight * POWER(0.5, age_days / ($3::integer)::float8)) AS raw_risk
			FROM located
			GROUP BY center_x_m, center_y_m
		), damped AS (
			SELECT *, LN(1.0 + raw_risk) AS damped_risk
			FROM raw
		), normalized AS (
			SELECT
				*,
				MIN(damped_risk) OVER () AS min_damped,
				MAX(damped_risk) OVER () AS max_damped
			FROM damped
		)
		INSERT INTO risk_zones (
			source_name, grid_size_m, grid_center_x_m, grid_center_y_m, zone_geom,
			incident_count, raw_risk, damped_risk, normalized_risk,
			recency_half_life_days, scored_at
		)
		SELECT
			$4,
			$1::integer,
			center_x_m,
			center_y_m,
			ST_MakeEnvelope(
				center_x_m - ($1::integer)::float8 / 2.0,
				center_y_m - ($1::integer)::float8 / 2.0,
				center_x_m + ($1::integer)::float8 / 2.0,
				center_y_m + ($1::integer)::float8 / 2.0,
				32619
			),
			incident_count,
			raw_risk,
			damped_risk,
			CASE
				WHEN max_damped = min_damped THEN 0.0
				ELSE (damped_risk - min_damped) / (max_damped - min_damped)
			END,
			$3::integer,
			$2
		FROM normalized`
	if _, err := tx.ExecContext(ctx, insertZones,
		options.GridSizeM, options.AsOf, options.RecencyHalfLifeDays, options.SourceName); err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("aggregate incidents into risk zones: %w", err)
	}

	const assignIncidents = `
		UPDATE crime_incidents AS incident
		SET risk_zone_id = zone.risk_zone_id
		FROM risk_zones AS zone
		WHERE incident.source_name = $1
		  AND zone.source_name = $1
		  AND zone.grid_size_m = $2::integer
		  AND ST_X(ST_SnapToGrid(ST_Transform(incident.location::geometry, 32619), ($2::integer)::float8)) = zone.grid_center_x_m
		  AND ST_Y(ST_SnapToGrid(ST_Transform(incident.location::geometry, 32619), ($2::integer)::float8)) = zone.grid_center_y_m`
	if _, err := tx.ExecContext(ctx, assignIncidents, options.SourceName, options.GridSizeM); err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("link incidents to risk zones: %w", err)
	}

	var summary model.RiskZoneBuildSummary
	summary.SourceName = options.SourceName
	summary.GridSizeM = options.GridSizeM
	summary.HalfLifeDays = options.RecencyHalfLifeDays
	summary.ScoredAt = options.AsOf
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(incident_count), 0),
		       COALESCE(MIN(normalized_risk), 0), COALESCE(MAX(normalized_risk), 0)
		FROM risk_zones WHERE source_name = $1`, options.SourceName).Scan(
		&summary.ZoneCount,
		&summary.IncidentCount,
		&summary.MinNormalized,
		&summary.MaxNormalized,
	); err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("read risk-zone build summary: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.RiskZoneBuildSummary{}, fmt.Errorf("commit risk-zone build: %w", err)
	}
	return summary, nil
}

func (r *RiskZoneRepository) LookupEdgeMidpoints(
	ctx context.Context,
	sourceName string,
	midpoints []model.Coordinates,
) ([]model.EdgeRisk, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
		ReadOnly:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("begin midpoint risk lookup: %w", err)
	}
	defer tx.Rollback()

	var gridSize sql.NullInt64
	var gridSizeCount int
	err = tx.QueryRowContext(ctx, `
		SELECT MIN(grid_size_m), COUNT(DISTINCT grid_size_m)
		FROM risk_zones
		WHERE source_name = $1`, sourceName).Scan(&gridSize, &gridSizeCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrRiskZonesNotBuilt
	}
	if err != nil {
		return nil, fmt.Errorf("read risk-zone grid settings: %w", err)
	}
	if gridSizeCount == 0 {
		return nil, ErrRiskZonesNotBuilt
	}
	if gridSizeCount != 1 {
		return nil, ErrRiskZoneGridInconsistent
	}
	gridSizeM := int(gridSize.Int64)

	longitudes := make([]float64, len(midpoints))
	latitudes := make([]float64, len(midpoints))
	for i, midpoint := range midpoints {
		longitudes[i] = midpoint.Longitude
		latitudes[i] = midpoint.Latitude
	}

	rows, err := tx.QueryContext(ctx, `
		WITH points AS (
			SELECT
				ord,
				ST_X(ST_SnapToGrid(projected, ($4::integer)::float8)) AS center_x_m,
				ST_Y(ST_SnapToGrid(projected, ($4::integer)::float8)) AS center_y_m
			FROM (
				SELECT
					ord,
					ST_Transform(ST_SetSRID(ST_MakePoint(lon, lat), 4326), 32619) AS projected
				FROM unnest($1::float8[], $2::float8[]) WITH ORDINALITY AS input(lon, lat, ord)
			) AS projected_points
		)
		SELECT points.ord, zone.risk_zone_id, zone.normalized_risk
		FROM points
		LEFT JOIN risk_zones AS zone
		  ON zone.source_name = $3
		 AND zone.grid_size_m = $4::integer
		 AND zone.grid_center_x_m = points.center_x_m
		 AND zone.grid_center_y_m = points.center_y_m
		ORDER BY points.ord`,
		pq.Array(longitudes), pq.Array(latitudes), sourceName, gridSizeM)
	if err != nil {
		return nil, fmt.Errorf("lookup midpoint risk zones: %w", err)
	}
	defer rows.Close()

	results := make([]model.EdgeRisk, len(midpoints))
	for i, midpoint := range midpoints {
		results[i] = model.EdgeRisk{EdgeIndex: i, Midpoint: midpoint}
	}
	for rows.Next() {
		var ordinal int
		var riskZoneID sql.NullInt64
		var normalizedRisk sql.NullFloat64
		if err := rows.Scan(&ordinal, &riskZoneID, &normalizedRisk); err != nil {
			return nil, fmt.Errorf("read midpoint risk result: %w", err)
		}
		if ordinal < 1 || ordinal > len(results) {
			return nil, fmt.Errorf("risk lookup returned invalid edge index %d", ordinal)
		}
		if riskZoneID.Valid && normalizedRisk.Valid {
			results[ordinal-1].RiskAvailable = true
			results[ordinal-1].RiskZoneID = &riskZoneID.Int64
			results[ordinal-1].NormalizedRisk = &normalizedRisk.Float64
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate midpoint risk results: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close midpoint risk results: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit midpoint risk lookup: %w", err)
	}
	return results, nil
}
