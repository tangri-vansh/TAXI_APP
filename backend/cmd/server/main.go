package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/lib/pq" // PostgreSQL driver (imported for its side-effects)
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
	// Initialize database connection
	initDB()
	defer db.Close() // Ensure the connection closes when main() exits

	// Register route
	http.HandleFunc("/health", healthHandler)

	fmt.Println("Server running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}