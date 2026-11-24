# Risk-zone build command

The command groups imported incidents into square grid cells in Boston's UTM
Zone 19N projection (EPSG:32619), computes the design's decayed weighted score,
log-damps and normalizes populated cells, and links each incident to its cell.
It replaces the current risk-zone set in a single transaction.

Apply `migrations/migrations/007_create_risk_zones.up.sql` after migration 006,
then run from the `backend` directory:

```sh
DATABASE_URL="${DATABASE_URL:?Set DATABASE_URL first}" \
go run ./cmd/build-risk-zones
```

Defaults are Boston PD (`boston_pd`), 500 m cells, and a 365-day recency
half-life. Override them with `-source`, `-grid-size-m`, or `-half-life-days`.
Set `-as-of` to a fixed RFC3339 timestamp
to reproduce a build; otherwise it uses the current UTC time.

Only cells containing imported incidents are stored. A missing cell therefore
means no mapped incident cell was produced; it must not automatically be
treated as verified zero risk during route integration.
