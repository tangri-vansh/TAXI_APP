package model

type Driver struct {
	DriverID    int64  `json:"driver_id"`
	UserID      int64  `json:"user_id"`
	Status      string `json:"status"`
	VehicleType string `json:"vehicle_type"`
	PlateNumber string `json:"plate_number"`
}

type Coordinates struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

type AvailableDriver struct {
	DriverID    int64       `json:"driver_id"`
	VehicleType string      `json:"vehicle_type"`
	PlateNumber string      `json:"plate_number"`
	Location    Coordinates `json:"location"`
	DistanceM   float64     `json:"distance_meters"`
}

type Ride struct {
	RideID      int64       `json:"ride_id"`
	RiderID     int64       `json:"rider_id"`
	DriverID    *int64      `json:"driver_id,omitempty"`
	Pickup      Coordinates `json:"pickup"`
	Destination Coordinates `json:"destination"`
	Status      string      `json:"status"`
	SafeMode    bool        `json:"safe_mode"`
}

type RideStop struct {
	StopID        int64       `json:"stop_id"`
	RideID        int64       `json:"ride_id"`
	Location      Coordinates `json:"location"`
	SequenceOrder int         `json:"sequence_order"`
	Status        string      `json:"status"`
}
