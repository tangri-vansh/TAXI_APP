CREATE TABLE ride_stops (
    stop_id BIGSERIAL PRIMARY KEY,
    ride_id BIGINT NOT NULL REFERENCES rides(ride_id),
    location GEOGRAPHY(POINT, 4326) NOT NULL,
    sequence_order INTEGER NOT NULL CHECK (sequence_order > 0),
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'COMPLETED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (ride_id, sequence_order)
);

CREATE INDEX ride_stops_ride_order_idx ON ride_stops (ride_id, sequence_order);
