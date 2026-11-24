CREATE TABLE routes (
    route_id BIGSERIAL PRIMARY KEY,
    ride_id BIGINT NOT NULL REFERENCES rides(ride_id),
    geometry JSONB NOT NULL CHECK (jsonb_typeof(geometry) = 'array'),
    distance_km DOUBLE PRECISION NOT NULL CHECK (distance_km > 0),
    eta_seconds DOUBLE PRECISION NOT NULL CHECK (eta_seconds > 0),
    risk_score DOUBLE PRECISION CHECK (risk_score IS NULL OR risk_score >= 0),
    mode TEXT NOT NULL CHECK (mode IN ('NORMAL', 'SAFE_MODE', 'NORMAL_FALLBACK')),
    safe_mode_used BOOLEAN NOT NULL DEFAULT FALSE,
    fallback BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_reason TEXT,
    normal_duration_seconds DOUBLE PRECISION,
    selected_beta DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX routes_ride_created_idx ON routes (ride_id, created_at DESC, route_id DESC);
