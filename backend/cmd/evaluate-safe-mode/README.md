# Safe Mode route evaluation

Run this command after importing Boston incidents and building the `boston_pd`
risk zones. It calculates Normal and Safe Mode routes for five fixed Boston
origin/destination pairs and writes CSV metrics to stdout. It does not modify
the database.

From the `backend` directory:

```sh
DATABASE_URL="${DATABASE_URL:?Set DATABASE_URL first}" \
VALHALLA_URL='http://localhost:8002' \
go run ./cmd/evaluate-safe-mode
```

The CSV reports normal and selected route ETA, percent ETA increase, risk
exposure, percent risk-exposure reduction, whether Safe Mode was selected, and
any fallback reason. `N/A` risk reduction means the normal route crosses at
least one edge without a Boston risk-zone score; it is not treated as zero
risk. The first completed run is recorded in `Docs/safe_mode_evaluation.md`.
The coordinate pairs are in `main.go` and should be kept fixed when comparing
evaluation runs.
