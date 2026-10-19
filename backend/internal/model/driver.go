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
