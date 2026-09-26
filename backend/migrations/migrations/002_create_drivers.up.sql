CREATE TABLE drivers (
    driver_id BIGSERIAL PRIMARY KEY,

    user_id BIGINT NOT NULL REFERENCES users(user_id),

    status TEXT NOT NULL DEFAULT 'AVAILABLE',

    vehicle_type TEXT NOT NULL,

    plate_number TEXT NOT NULL UNIQUE
);