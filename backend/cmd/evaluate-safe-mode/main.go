package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	"safe-ride/internal/model"
	"safe-ride/internal/repository"
	"safe-ride/internal/routing"
	"safe-ride/internal/service"
)

const sourceName = "boston_pd"

type routePair struct {
	name        string
	origin      model.Coordinates
	destination model.Coordinates
}

var evaluationPairs = []routePair{
	{name: "common_fenway", origin: model.Coordinates{Latitude: 42.3550, Longitude: -71.0650}, destination: model.Coordinates{Latitude: 42.3467, Longitude: -71.0972}},
	{name: "south_end_charlestown", origin: model.Coordinates{Latitude: 42.3410, Longitude: -71.0760}, destination: model.Coordinates{Latitude: 42.3780, Longitude: -71.0600}},
	{name: "roxbury_north_end", origin: model.Coordinates{Latitude: 42.3290, Longitude: -71.0830}, destination: model.Coordinates{Latitude: 42.3640, Longitude: -71.0540}},
	{name: "east_boston_back_bay", origin: model.Coordinates{Latitude: 42.3700, Longitude: -71.0390}, destination: model.Coordinates{Latitude: 42.3500, Longitude: -71.0800}},
	{name: "dorchester_seaport", origin: model.Coordinates{Latitude: 42.3150, Longitude: -71.0580}, destination: model.Coordinates{Latitude: 42.3500, Longitude: -71.0440}},
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL must contain the PostgreSQL connection string")
	}
	valhallaURL := os.Getenv("VALHALLA_URL")
	if valhallaURL == "" {
		valhallaURL = "http://localhost:8002"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	riskZones := service.NewRiskZoneService(repository.NewRiskZoneRepository(db))
	engine := routing.NewSafeModeValhallaEngine(routing.NewValhallaClient(valhallaURL), riskZones, sourceName)

	fmt.Println("pair,normal_eta_seconds,safe_eta_seconds,eta_increase_percent,normal_risk_exposure,safe_risk_exposure,risk_reduction_percent,safe_mode_used,fallback_reason")
	var failures int
	for _, pair := range evaluationPairs {
		if err := evaluatePair(ctx, engine, pair); err != nil {
			failures++
			log.Printf("pair %s failed: %v", pair.name, err)
		}
	}
	if failures > 0 {
		log.Fatalf("Safe Mode evaluation completed with %d failed route pairs", failures)
	}
}

func evaluatePair(ctx context.Context, engine *routing.SafeModeValhallaEngine, pair routePair) error {
	locations := []routing.Coordinate{
		{Latitude: pair.origin.Latitude, Longitude: pair.origin.Longitude},
		{Latitude: pair.destination.Latitude, Longitude: pair.destination.Longitude},
	}
	normal, err := engine.RouteForCost(ctx, locations, nil)
	if err != nil {
		return fmt.Errorf("calculate Normal Mode route: %w", err)
	}
	result, err := engine.Plan(ctx, locations, []float64{120, 60, 30, 15, 7.5})
	if err != nil {
		return fmt.Errorf("plan Safe Mode route: %w", err)
	}

	etaIncrease := percentageChange(normal.Route.DurationSec, result.Route.DurationSec)
	riskReduction := "N/A"
	if normal.RiskCoverageComplete {
		riskReduction = fmt.Sprintf("%.2f", percentageReduction(normal.RiskExposure, result.RiskExposure))
	}
	fallbackReason := result.FallbackReason
	fmt.Printf("%s,%.3f,%.3f,%.2f,%.4f,%.4f,%s,%t,%q\n",
		pair.name, normal.Route.DurationSec, result.Route.DurationSec, etaIncrease,
		normal.RiskExposure, result.RiskExposure, riskReduction, result.UsedSafeMode, fallbackReason)
	return nil
}

func percentageChange(baseline, selected float64) float64 {
	if baseline <= 0 {
		return 0
	}
	return (selected - baseline) / baseline * 100
}

func percentageReduction(baseline, selected float64) float64 {
	if baseline <= 0 {
		return 0
	}
	return (baseline - selected) / baseline * 100
}
