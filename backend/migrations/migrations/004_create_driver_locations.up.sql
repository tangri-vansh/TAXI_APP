CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE driver_locations (
    location_id BIGSERIAL PRIMARY KEY,
    driver_id BIGINT NOT NULL REFERENCES drivers(driver_id),
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX driver_locations_driver_timestamp_idx
    ON driver_locations (driver_id, timestamp DESC);
