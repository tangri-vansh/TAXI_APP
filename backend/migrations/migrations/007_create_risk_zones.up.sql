CREATE TABLE risk_zones (
    risk_zone_id BIGSERIAL PRIMARY KEY,
    source_name TEXT NOT NULL,
    grid_size_m INTEGER NOT NULL CHECK (grid_size_m > 0),
    grid_center_x_m DOUBLE PRECISION NOT NULL,
    grid_center_y_m DOUBLE PRECISION NOT NULL,
    zone_geom GEOMETRY(POLYGON, 32619) NOT NULL,
    incident_count BIGINT NOT NULL CHECK (incident_count > 0),
    raw_risk DOUBLE PRECISION NOT NULL CHECK (raw_risk >= 0),
    damped_risk DOUBLE PRECISION NOT NULL CHECK (damped_risk >= 0),
    normalized_risk DOUBLE PRECISION NOT NULL CHECK (normalized_risk BETWEEN 0 AND 1),
    recency_half_life_days INTEGER NOT NULL CHECK (recency_half_life_days > 0),
    scored_at TIMESTAMPTZ NOT NULL,
    UNIQUE (source_name, grid_size_m, grid_center_x_m, grid_center_y_m)
);

CREATE INDEX risk_zones_geometry_gist_idx
    ON risk_zones USING GIST (zone_geom);

CREATE INDEX risk_zones_normalized_risk_idx
    ON risk_zones (normalized_risk);

ALTER TABLE crime_incidents
    ADD COLUMN risk_zone_id BIGINT REFERENCES risk_zones(risk_zone_id) ON DELETE SET NULL;

CREATE INDEX crime_incidents_risk_zone_idx
    ON crime_incidents (risk_zone_id);
