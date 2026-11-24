package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	"safe-ride/internal/repository"
	"safe-ride/internal/service"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL must contain the PostgreSQL connection string")
	}

	gridSizeM := flag.Int("grid-size-m", 500, "risk-zone grid-cell side length in meters")
	halfLifeDays := flag.Int("half-life-days", 365, "configurable recency half-life in days")
	sourceName := flag.String("source", "boston_pd", "incident source to aggregate")
	asOfValue := flag.String("as-of", "", "risk-score reference time (RFC3339); defaults to current UTC time")
	flag.Parse()

	asOf := time.Time{}
	if *asOfValue != "" {
		parsed, err := time.Parse(time.RFC3339, *asOfValue)
		if err != nil {
			log.Fatalf("invalid -as-of value: %v", err)
		}
		asOf = parsed
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	repo := repository.NewRiskZoneRepository(db)
	appService := service.NewRiskZoneService(repo)
	summary, err := appService.Build(ctx, *gridSizeM, *halfLifeDays, *sourceName, asOf)
	if err != nil {
		log.Fatalf("build risk zones: %v", err)
	}
	fmt.Printf("Risk-zone build complete: source=%s zones=%d incidents=%d grid_size_m=%d half_life_days=%d normalized_risk=[%.6f, %.6f] scored_at=%s\n",
		summary.SourceName,
		summary.ZoneCount,
		summary.IncidentCount,
		summary.GridSizeM,
		summary.HalfLifeDays,
		summary.MinNormalized,
		summary.MaxNormalized,
		summary.ScoredAt.Format(time.RFC3339),
	)
}
