package model

type DriverLocation struct {
	LocationID int64   `json:"location_id"`
	DriverID   int64   `json:"driver_id"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}
