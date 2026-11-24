package model

import "time"

type RiskZoneBuildOptions struct {
	SourceName          string
	GridSizeM           int
	RecencyHalfLifeDays int
	AsOf                time.Time
}

type RiskZoneBuildSummary struct {
	SourceName    string    `json:"source_name"`
	ZoneCount     int64     `json:"zone_count"`
	IncidentCount int64     `json:"incident_count"`
	GridSizeM     int       `json:"grid_size_m"`
	HalfLifeDays  int       `json:"recency_half_life_days"`
	ScoredAt      time.Time `json:"scored_at"`
	MinNormalized float64   `json:"min_normalized_risk"`
	MaxNormalized float64   `json:"max_normalized_risk"`
}

type EdgeRisk struct {
	EdgeIndex      int         `json:"edge_index"`
	Midpoint       Coordinates `json:"midpoint"`
	RiskAvailable  bool        `json:"risk_available"`
	RiskZoneID     *int64      `json:"risk_zone_id,omitempty"`
	NormalizedRisk *float64    `json:"normalized_risk,omitempty"`
}

// RiskZonePolygon is the WGS84 outline and normalized score sent to the
// custom Valhalla costing function for edge-midpoint lookup during A*.
type RiskZonePolygon struct {
	NormalizedRisk float64       `json:"normalized_risk"`
	Vertices       []Coordinates `json:"vertices"`
}
