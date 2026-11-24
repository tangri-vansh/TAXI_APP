package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"safe-ride/internal/model"
)

const valhallaPolylinePrecision = 6

var ErrInvalidTraceAttributes = errors.New("Valhalla trace attributes response is invalid")

type SafeModeRiskZoneProvider interface {
	RoutingPolygons(ctx context.Context, sourceName string) ([]model.RiskZonePolygon, error)
	LookupEdgeMidpoints(ctx context.Context, sourceName string, midpoints []model.Coordinates) ([]model.EdgeRisk, error)
}

// SafeModeValhallaEngine connects the Go planner to the custom Valhalla
// costing request and evaluates returned graph edges using the risk-zone data.
type SafeModeValhallaEngine struct {
	client     *ValhallaClient
	riskZones  SafeModeRiskZoneProvider
	sourceName string
}

func NewSafeModeValhallaEngine(
	client *ValhallaClient,
	riskZones SafeModeRiskZoneProvider,
	sourceName string,
) *SafeModeValhallaEngine {
	return &SafeModeValhallaEngine{
		client:     client,
		riskZones:  riskZones,
		sourceName: strings.TrimSpace(sourceName),
	}
}

// Plan invokes the Go Safe Mode retry planner with this Valhalla-backed
// engine, connecting baseline, beta retries, and fallback to the route service.
func (e *SafeModeValhallaEngine) Plan(
	ctx context.Context,
	locations []Coordinate,
	betaValues []float64,
) (*SafeModeResult, error) {
	return PlanSafeMode(ctx, e, locations, SafeModeConfig{BetaValues: betaValues})
}

func (e *SafeModeValhallaEngine) RouteForCost(
	ctx context.Context,
	locations []Coordinate,
	beta *float64,
) (*RouteEvaluation, error) {
	if e == nil || e.client == nil || e.riskZones == nil || e.sourceName == "" {
		return nil, ErrInvalidSafeModeConfig
	}

	var polygons []model.RiskZonePolygon
	var err error
	if beta != nil {
		if math.IsNaN(*beta) || math.IsInf(*beta, 0) || *beta <= 0 {
			return nil, ErrInvalidSafeModeConfig
		}
		polygons, err = e.riskZones.RoutingPolygons(ctx, e.sourceName)
		if err != nil {
			return nil, fmt.Errorf("load risk zones for Valhalla: %w", err)
		}
	}

	route, err := e.client.route(ctx, locations, beta, polygons)
	if err != nil {
		return nil, err
	}
	evaluation, err := e.evaluateRoute(ctx, route)
	if err != nil {
		return nil, err
	}
	return evaluation, nil
}

func (e *SafeModeValhallaEngine) evaluateRoute(ctx context.Context, route *Route) (*RouteEvaluation, error) {
	allMidpoints := make([]model.Coordinates, 0)
	for _, leg := range route.Legs {
		points, err := decodePolyline(leg.Shape, valhallaPolylinePrecision)
		if err != nil {
			return nil, fmt.Errorf("decode Valhalla route shape: %w", err)
		}
		traceEdges, err := e.traceEdges(ctx, points)
		if err != nil {
			return nil, err
		}
		for _, edge := range traceEdges {
			if edge.BeginShapeIndex < 0 || edge.EndShapeIndex < edge.BeginShapeIndex || edge.EndShapeIndex >= len(points) {
				return nil, ErrInvalidTraceAttributes
			}
			midpoint, ok := polylineMidpoint(points[edge.BeginShapeIndex : edge.EndShapeIndex+1])
			if !ok {
				return nil, ErrInvalidTraceAttributes
			}
			allMidpoints = append(allMidpoints, model.Coordinates{
				Latitude: midpoint.Latitude, Longitude: midpoint.Longitude,
			})
		}
	}
	if len(allMidpoints) == 0 {
		return nil, ErrInvalidTraceAttributes
	}

	edgeRisks, err := e.riskZones.LookupEdgeMidpoints(ctx, e.sourceName, allMidpoints)
	if err != nil {
		return nil, fmt.Errorf("lookup routed edge midpoint risks: %w", err)
	}
	if len(edgeRisks) != len(allMidpoints) {
		return nil, ErrInvalidTraceAttributes
	}

	evaluation := &RouteEvaluation{Route: route, RiskCoverageComplete: true}
	for _, edgeRisk := range edgeRisks {
		if !edgeRisk.RiskAvailable || edgeRisk.NormalizedRisk == nil {
			evaluation.RiskCoverageComplete = false
			continue
		}
		evaluation.RiskExposure += *edgeRisk.NormalizedRisk
	}
	return evaluation, nil
}

type traceEdge struct {
	BeginShapeIndex int `json:"begin_shape_index"`
	EndShapeIndex   int `json:"end_shape_index"`
}

func (e *SafeModeValhallaEngine) traceEdges(ctx context.Context, points []Coordinate) ([]traceEdge, error) {
	if len(points) < 2 {
		return nil, ErrInvalidTraceAttributes
	}
	requestBody := struct {
		Shape          []Coordinate `json:"shape"`
		Costing        string       `json:"costing"`
		CostingOptions any          `json:"costing_options"`
		ShapeMatch     string       `json:"shape_match"`
		Filters        struct {
			Attributes []string `json:"attributes"`
			Action     string   `json:"action"`
		} `json:"filters"`
	}{
		Shape:          points,
		Costing:        "auto",
		CostingOptions: map[string]any{"auto": map[string]any{}},
		ShapeMatch:     "map_snap",
	}
	requestBody.Filters.Attributes = []string{"edge.begin_shape_index", "edge.end_shape_index"}
	requestBody.Filters.Action = "include"
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode Valhalla trace request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.client.baseURL+"/trace_attributes", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Valhalla trace request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := e.client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Valhalla trace attributes: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		if readErr != nil {
			return nil, fmt.Errorf("Valhalla trace attributes returned HTTP %d", response.StatusCode)
		}
		return nil, fmt.Errorf("Valhalla trace attributes returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(errorBody)))
	}
	var result struct {
		Edges []traceEdge `json:"edges"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Valhalla trace attributes: %w", err)
	}
	if len(result.Edges) == 0 {
		return nil, ErrInvalidTraceAttributes
	}
	return result.Edges, nil
}

func decodePolyline(encoded string, precision int) ([]Coordinate, error) {
	if encoded == "" || precision < 0 || precision > 9 {
		return nil, ErrInvalidTraceAttributes
	}
	coordinates := make([]Coordinate, 0, len(encoded)/4)
	lat, lon := int64(0), int64(0)
	for index := 0; index < len(encoded); {
		decodedLat, next, err := decodePolylineValue(encoded, index)
		if err != nil {
			return nil, err
		}
		index = next
		decodedLon, next, err := decodePolylineValue(encoded, index)
		if err != nil {
			return nil, err
		}
		index = next
		lat += decodedLat
		lon += decodedLon
		factor := math.Pow10(precision)
		coordinates = append(coordinates, Coordinate{
			Latitude: float64(lat) / factor, Longitude: float64(lon) / factor,
		})
	}
	if len(coordinates) < 2 {
		return nil, ErrInvalidTraceAttributes
	}
	return coordinates, nil
}

func decodePolylineValue(encoded string, index int) (int64, int, error) {
	var result uint64
	var shift uint
	for {
		if index >= len(encoded) || shift > 60 {
			return 0, index, ErrInvalidTraceAttributes
		}
		value := uint64(encoded[index] - 63)
		index++
		result |= (value & 0x1f) << shift
		shift += 5
		if value < 0x20 {
			break
		}
	}
	decoded := int64(result >> 1)
	if result&1 != 0 {
		decoded = ^decoded
	}
	return decoded, index, nil
}

func polylineMidpoint(points []Coordinate) (Coordinate, bool) {
	if len(points) == 0 {
		return Coordinate{}, false
	}
	if len(points) == 1 {
		return points[0], true
	}
	lengths := make([]float64, len(points)-1)
	total := 0.0
	for i := 1; i < len(points); i++ {
		lengths[i-1] = haversineMeters(points[i-1], points[i])
		total += lengths[i-1]
	}
	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return Coordinate{}, false
	}
	remaining := total / 2
	for i, segmentLength := range lengths {
		if remaining <= segmentLength {
			fraction := remaining / segmentLength
			return Coordinate{
				Latitude:  points[i].Latitude + (points[i+1].Latitude-points[i].Latitude)*fraction,
				Longitude: points[i].Longitude + (points[i+1].Longitude-points[i].Longitude)*fraction,
			}, true
		}
		remaining -= segmentLength
	}
	return points[len(points)-1], true
}

func haversineMeters(a, b Coordinate) float64 {
	const earthRadiusM = 6371008.8
	lat1 := a.Latitude * math.Pi / 180
	lat2 := b.Latitude * math.Pi / 180
	deltaLat := (b.Latitude - a.Latitude) * math.Pi / 180
	deltaLon := (b.Longitude - a.Longitude) * math.Pi / 180
	term := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(term))
}
