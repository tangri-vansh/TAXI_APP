package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"safe-ride/internal/model"
)

// Coordinate is a geographic point in decimal degrees.
type Coordinate struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// Route contains road-network results returned by Valhalla.
type Route struct {
	DistanceKM  float64
	DurationSec float64
	Legs        []RouteLeg
}

// RouteLeg is the road route between two successive requested locations.
type RouteLeg struct {
	DistanceKM  float64
	DurationSec float64
	Shape       string
}

// ValhallaClient calls a Valhalla HTTP routing service.
type ValhallaClient struct {
	baseURL    string
	httpClient *http.Client
}

// ErrNoRouteFound indicates that Valhalla accepted the request but found no
// path through the available graph edges (including Safe Mode risk coverage).
var ErrNoRouteFound = errors.New("Valhalla found no route")

func NewValhallaClient(baseURL string) *ValhallaClient {
	return &ValhallaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Route calculates a normal time-optimized road route through locations in the
// supplied order. Safe Mode uses the same endpoint with its custom costing
// options through the private route method.
func (c *ValhallaClient) Route(ctx context.Context, locations []Coordinate) (*Route, error) {
	return c.route(ctx, locations, nil, nil)
}

func (c *ValhallaClient) route(
	ctx context.Context,
	locations []Coordinate,
	beta *float64,
	riskZones []model.RiskZonePolygon,
) (*Route, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("Valhalla URL is not configured")
	}
	if len(locations) < 2 {
		return nil, fmt.Errorf("at least two route locations are required")
	}

	requestBody := struct {
		Locations      []Coordinate `json:"locations"`
		Costing        string       `json:"costing"`
		CostingOptions any          `json:"costing_options,omitempty"`
	}{
		Locations: locations,
		Costing:   "auto",
		// This Valhalla build expects the costing-specific options object even
		// when using the default Auto settings.
		CostingOptions: map[string]any{"auto": map[string]any{}},
	}
	if beta != nil {
		requestZones := make([]struct {
			NormalizedRisk float64      `json:"normalized_risk"`
			Vertices       []Coordinate `json:"vertices"`
		}, 0, len(riskZones))
		for _, zone := range riskZones {
			vertices := make([]Coordinate, 0, len(zone.Vertices))
			for _, vertex := range zone.Vertices {
				vertices = append(vertices, Coordinate{Latitude: vertex.Latitude, Longitude: vertex.Longitude})
			}
			requestZones = append(requestZones, struct {
				NormalizedRisk float64      `json:"normalized_risk"`
				Vertices       []Coordinate `json:"vertices"`
			}{NormalizedRisk: zone.NormalizedRisk, Vertices: vertices})
		}
		requestBody.CostingOptions = map[string]any{
			"auto": map[string]any{
				"safe_mode_beta":       *beta,
				"safe_mode_risk_zones": requestZones,
			},
		}
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode Valhalla route request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/route", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Valhalla route request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Valhalla route service: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr != nil {
			return nil, fmt.Errorf("Valhalla returned HTTP %d", response.StatusCode)
		}
		var valhallaError struct {
			ErrorCode int `json:"error_code"`
		}
		if json.Unmarshal(errorBody, &valhallaError) == nil && valhallaError.ErrorCode == 442 {
			return nil, ErrNoRouteFound
		}
		return nil, fmt.Errorf("Valhalla returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(errorBody)))
	}

	var valhallaResponse struct {
		Trip struct {
			Summary struct {
				Length float64 `json:"length"`
				Time   float64 `json:"time"`
			} `json:"summary"`
			Legs []struct {
				Summary struct {
					Length float64 `json:"length"`
					Time   float64 `json:"time"`
				} `json:"summary"`
				Shape string `json:"shape"`
			} `json:"legs"`
		} `json:"trip"`
	}
	if err := json.NewDecoder(response.Body).Decode(&valhallaResponse); err != nil {
		return nil, fmt.Errorf("decode Valhalla route response: %w", err)
	}
	if len(valhallaResponse.Trip.Legs) == 0 {
		return nil, fmt.Errorf("Valhalla route response did not include any route legs")
	}

	result := &Route{
		DistanceKM:  valhallaResponse.Trip.Summary.Length,
		DurationSec: valhallaResponse.Trip.Summary.Time,
		Legs:        make([]RouteLeg, 0, len(valhallaResponse.Trip.Legs)),
	}
	for _, leg := range valhallaResponse.Trip.Legs {
		result.Legs = append(result.Legs, RouteLeg{
			DistanceKM:  leg.Summary.Length,
			DurationSec: leg.Summary.Time,
			Shape:       leg.Shape,
		})
	}
	return result, nil
}
