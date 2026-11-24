package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/lib/pq"

	"safe-ride/internal/handler"
	"safe-ride/internal/repository"
	"safe-ride/internal/routing"
	"safe-ride/internal/service"
)

// Global database connection pool
var db *sql.DB

func initDB() {
	connStr := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connStr == "" {
		log.Fatal("DATABASE_URL must contain the PostgreSQL connection string")
	}

	var err error
	// Open opens a database specified by its driver name and driver-specific data source name
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}

	// Ping verifies a connection to the database is still alive
	err = db.Ping()
	if err != nil {
		log.Fatalf("Cannot connect to PostgreSQL: %v", err)
	}

	fmt.Println("Successfully connected to PostgreSQL database!")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Verify database connection during health check
	dbStatus := "connected"
	if err := db.Ping(); err != nil {
		dbStatus = "disconnected"
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"database": dbStatus,
	})
}

func main() {
	// 1. Initialize the database connection.
	initDB()
	defer db.Close()

	// 2. Create the user feature's dependency chain.
	userRepository := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepository)
	userHandler := handler.NewUserHandler(userService)
	driverRepository := repository.NewDriverRepository(db)
	driverService := service.NewDriverService(driverRepository)
	driverLocationRepository := repository.NewDriverLocationRepository(db)
	driverLocationService := service.NewDriverLocationService(driverLocationRepository)
	driverHandler := handler.NewDriverHandler(driverService, driverLocationService)
	rideRepository := repository.NewRideRepository(db)
	rideService := service.NewRideService(rideRepository)
	riskZoneRepository := repository.NewRiskZoneRepository(db)
	riskZoneService := service.NewRiskZoneService(riskZoneRepository)
	valhallaURL := os.Getenv("VALHALLA_URL")
	if valhallaURL == "" {
		valhallaURL = "http://localhost:8002"
	}
	valhallaClient := routing.NewValhallaClient(valhallaURL)
	safeModeEngine := routing.NewSafeModeValhallaEngine(valhallaClient, riskZoneService, "boston_pd")
	routeRepository := repository.NewRouteRepository(db)
	rideRouteService := service.NewRideRouteService(rideRepository, routeRepository, valhallaClient, safeModeEngine)
	rideStopService := service.NewRideStopService(rideRepository, rideRouteService)
	rideHandler := handler.NewRideHandler(rideService, rideRouteService, rideStopService)

	// 3. Create the HTTP router.
	mux := http.NewServeMux()

	// 4. Register the existing health endpoint.
	mux.HandleFunc("GET /health", healthHandler)

	// 5. Register the user creation endpoint.
	mux.HandleFunc("POST /users", userHandler.CreateUser)
	mux.HandleFunc("POST /drivers", driverHandler.CreateDriver)
	mux.HandleFunc("POST /drivers/{driver_id}/location", driverHandler.UpdateLocation)
	mux.HandleFunc("GET /drivers/available", driverHandler.GetAvailableDrivers)
	mux.HandleFunc("POST /rides", rideHandler.CreateRide)
	mux.HandleFunc("GET /rides/{ride_id}", rideHandler.GetRide)
	mux.HandleFunc("GET /rides/{ride_id}/route", rideHandler.GetRoute)
	mux.HandleFunc("POST /rides/{ride_id}/accept", rideHandler.AcceptRide)
	mux.HandleFunc("POST /rides/{ride_id}/start", rideHandler.StartRide)
	mux.HandleFunc("POST /rides/{ride_id}/complete", rideHandler.CompleteRide)
	mux.HandleFunc("POST /rides/{ride_id}/cancel", rideHandler.CancelRide)
	mux.HandleFunc("POST /rides/{ride_id}/stops", rideHandler.AddStop)

	// 6. Start the HTTP server.
	fmt.Println("Server running on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
