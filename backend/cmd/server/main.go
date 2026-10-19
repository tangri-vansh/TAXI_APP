package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq"

	"safe-ride/internal/handler"
	"safe-ride/internal/repository"
	"safe-ride/internal/service"
)

// Global database connection pool
var db *sql.DB

func initDB() {
	// Connection cstring matching credentials from docker-compose.yml
	//ssl mode needs to change when deploying
	connStr := "host=localhost port=5432 user=postgres password=postgrespassword dbname=safe_ride sslmode=disable"

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

	// 3. Create the HTTP router.
	mux := http.NewServeMux()

	// 4. Register the existing health endpoint.
	mux.HandleFunc("GET /health", healthHandler)

	// 5. Register the user creation endpoint.
	mux.HandleFunc("POST /users", userHandler.CreateUser)

	// 6. Start the HTTP server.
	fmt.Println("Server running on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
