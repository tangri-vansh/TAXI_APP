CREATE TABLE crime_incidents (
    crime_incident_id BIGSERIAL PRIMARY KEY,
    source_name TEXT NOT NULL,
    source_incident_number TEXT NOT NULL,
    offense_code TEXT NOT NULL,
    offense_description TEXT NOT NULL,
    risk_category TEXT NOT NULL,
    risk_weight DOUBLE PRECISION NOT NULL CHECK (risk_weight >= 0),
    occurred_at TIMESTAMPTZ NOT NULL,
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    source_snapshot_date DATE NOT NULL,
    UNIQUE (source_name, source_incident_number, offense_code)
);

CREATE INDEX crime_incidents_location_gist_idx
    ON crime_incidents USING GIST (location);

CREATE INDEX crime_incidents_occurred_at_idx
    ON crime_incidents (occurred_at);

CREATE INDEX crime_incidents_risk_category_idx
    ON crime_incidents (risk_category);
