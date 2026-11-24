package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type offenseMapping struct {
	category string
	weight   float64
	include  bool
}

func main() {
	csvPath := flag.String("csv", "../tmpivat4nah.csv", "path to the Boston incident CSV")
	mappingPath := flag.String("mapping", "data/boston_offense_mapping.csv", "path to the offense mapping CSV")
	snapshot := flag.String("snapshot-date", "", "date this source snapshot was downloaded (YYYY-MM-DD)")
	sourceName := flag.String("source", "boston_pd", "source identifier")
	flag.Parse()

	if *snapshot == "" {
		log.Fatal("-snapshot-date is required (YYYY-MM-DD)")
	}
	snapshotDate, err := time.Parse("2006-01-02", *snapshot)
	if err != nil {
		log.Fatalf("invalid -snapshot-date: %v", err)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL must contain the PostgreSQL connection string")
	}

	mapping, err := loadMapping(*mappingPath)
	if err != nil {
		log.Fatalf("load offense mapping: %v", err)
	}
	input, err := os.Open(*csvPath)
	if err != nil {
		log.Fatalf("open source CSV: %v", err)
	}
	defer input.Close()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	if err := importCSV(db, input, mapping, *sourceName, snapshotDate); err != nil {
		log.Fatal(err)
	}
}

func loadMapping(path string) (map[string]offenseMapping, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	columns := indexColumns(header)
	for _, name := range []string{"offense_code", "risk_category", "risk_weight", "include"} {
		if _, ok := columns[name]; !ok {
			return nil, fmt.Errorf("mapping CSV is missing %q column", name)
		}
	}

	result := make(map[string]offenseMapping)
	for line := 2; ; line++ {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("mapping row %d: %w", line, err)
		}
		code := strings.TrimSpace(row[columns["offense_code"]])
		weight, err := strconv.ParseFloat(strings.TrimSpace(row[columns["risk_weight"]]), 64)
		if err != nil || weight < 0 {
			return nil, fmt.Errorf("mapping row %d has invalid risk_weight", line)
		}
		include, err := strconv.ParseBool(strings.TrimSpace(row[columns["include"]]))
		if err != nil {
			return nil, fmt.Errorf("mapping row %d has invalid include flag", line)
		}
		category := strings.TrimSpace(row[columns["risk_category"]])
		if include && category == "" {
			return nil, fmt.Errorf("mapping row %d includes code %s without a category", line, code)
		}
		if _, exists := result[code]; exists || code == "" {
			return nil, fmt.Errorf("mapping row %d has duplicate or empty offense_code %q", line, code)
		}
		result[code] = offenseMapping{category: category, weight: weight, include: include}
	}
	return result, nil
}

func importCSV(db *sql.DB, input io.Reader, mapping map[string]offenseMapping, source string, snapshotDate time.Time) error {
	reader := csv.NewReader(input)
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read source CSV header: %w", err)
	}
	columns := indexColumns(header)
	for _, name := range []string{"INCIDENT_NUMBER", "OFFENSE_CODE", "OFFENSE_DESCRIPTION", "OCCURRED_ON_DATE", "Lat", "Long"} {
		if _, ok := columns[name]; !ok {
			return fmt.Errorf("source CSV is missing %q column", name)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO crime_incidents (
			source_name, source_incident_number, offense_code, offense_description,
			risk_category, risk_weight, occurred_at, location, source_snapshot_date
		) VALUES ($1,$2,$3,$4,$5,$6,$7,ST_SetSRID(ST_MakePoint($8,$9),4326)::geography,$10)
		ON CONFLICT (source_name, source_incident_number, offense_code) DO UPDATE SET
			offense_description = EXCLUDED.offense_description,
			risk_category = EXCLUDED.risk_category,
			risk_weight = EXCLUDED.risk_weight,
			occurred_at = EXCLUDED.occurred_at,
			location = EXCLUDED.location,
			source_snapshot_date = EXCLUDED.source_snapshot_date`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	var rowsRead, upserted, excluded, invalidLocation int64
	categoryCounts := make(map[string]int64)
	for line := 2; ; line++ {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("source CSV row %d: %w", line, err)
		}
		rowsRead++
		code := strings.TrimSpace(row[columns["OFFENSE_CODE"]])
		classification, ok := mapping[code]
		if !ok {
			return fmt.Errorf("offense code %q at source row %d has no explicit mapping; refusing partial import", code, line)
		}
		if !classification.include {
			excluded++
			continue
		}

		lat, latErr := strconv.ParseFloat(strings.TrimSpace(row[columns["Lat"]]), 64)
		lon, lonErr := strconv.ParseFloat(strings.TrimSpace(row[columns["Long"]]), 64)
		if latErr != nil || lonErr != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
			invalidLocation++
			continue
		}
		occurredAt, err := parseOccurredAt(strings.TrimSpace(row[columns["OCCURRED_ON_DATE"]]))
		if err != nil {
			return fmt.Errorf("invalid occurrence date at source row %d: %w", line, err)
		}
		incidentNumber := strings.TrimSpace(row[columns["INCIDENT_NUMBER"]])
		if incidentNumber == "" {
			return fmt.Errorf("empty incident number at source row %d", line)
		}
		if _, err := stmt.Exec(source, incidentNumber, code,
			strings.TrimSpace(row[columns["OFFENSE_DESCRIPTION"]]), classification.category,
			classification.weight, occurredAt, lon, lat, snapshotDate); err != nil {
			return fmt.Errorf("insert source row %d: %w", line, err)
		}
		upserted++
		categoryCounts[classification.category]++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import: %w", err)
	}
	log.Printf("Boston import complete: rows_read=%d upserted=%d excluded_by_mapping=%d invalid_coordinates=%d snapshot_date=%s",
		rowsRead, upserted, excluded, invalidLocation, snapshotDate.Format("2006-01-02"))
	for category, count := range categoryCounts {
		log.Printf("imported category %s: %d rows", category, count)
	}
	return nil
}

func parseOccurredAt(value string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", value)
}

func indexColumns(header []string) map[string]int {
	result := make(map[string]int, len(header))
	for i, name := range header {
		result[strings.TrimSpace(name)] = i
	}
	return result
}
