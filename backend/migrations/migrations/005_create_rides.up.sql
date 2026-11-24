CREATE TABLE rides (
    ride_id BIGSERIAL PRIMARY KEY,
    rider_id BIGINT NOT NULL REFERENCES users(user_id),
    driver_id BIGINT REFERENCES drivers(driver_id),
    pickup GEOGRAPHY(POINT, 4326) NOT NULL,
    destination GEOGRAPHY(POINT, 4326) NOT NULL,
    status TEXT NOT NULL DEFAULT 'REQUESTED'
        CHECK (status IN ('REQUESTED', 'ACCEPTED', 'ONGOING', 'COMPLETED', 'CANCELLED')),
    safe_mode BOOLEAN NOT NULL DEFAULT FALSE,
    CHECK (NOT ST_Equals(pickup::geometry, destination::geometry))
);

CREATE UNIQUE INDEX rides_one_active_per_rider_unique_idx
    ON rides (rider_id)
    WHERE status IN ('REQUESTED', 'ACCEPTED', 'ONGOING');
