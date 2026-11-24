package model

import "time"

type RouteLeg struct {
	DistanceKM      float64 `json:"distance_km"`
	DurationSeconds float64 `json:"duration_seconds"`
	Shape           string  `json:"shape"`
}

// RouteSnapshot persists one generated route for a ride. Geometry contains
// Valhalla's encoded polyline and measurements for each route leg.
type RouteSnapshot struct {
	RouteID            int64      `json:"route_id"`
	RideID             int64      `json:"ride_id"`
	Mode               string     `json:"mode"`
	DistanceKM         float64    `json:"distance_km"`
	DurationSeconds    float64    `json:"duration_seconds"`
	Geometry           []RouteLeg `json:"geometry"`
	RiskExposure       *float64   `json:"risk_exposure,omitempty"`
	SafeModeUsed       bool       `json:"safe_mode_used"`
	Fallback           bool       `json:"fallback"`
	FallbackReason     string     `json:"fallback_reason,omitempty"`
	NormalDurationSecs *float64   `json:"normal_duration_seconds,omitempty"`
	SelectedBeta       *float64   `json:"selected_beta,omitempty"`
	CreatedAt          time.Time  `json:"created_at,omitempty"`
}
