# Safety-Aware Ride Hailing App

Go and PostgreSQL/PostGIS backend for a ride-hailing application with opt-in Safe Mode routing. Valhalla provides road routes. Safe Mode is designed to use historical reported-incident risk while enforcing a maximum 20% travel-time increase when a qualifying route exists.

## V1 pilot data region

The current pilot region is Boston. The local crime-data snapshot is `tmpivat4nah.csv`, containing Boston Police incident reports. See [the finalized Safe Mode design](Docs/design-doc_risk_fin.md) and [the Boston offense mapping inventory](Docs/boston_offense_mapping.md) for its source fields, limitations, and mapping status.

This data represents reported incidents, not all crime or personal safety. Some records have no usable coordinates, and mapped locations should not be treated as exact addresses. Offense assignments use the deterministic `OFFENSE_CODE` mapping in `backend/data/boston_offense_mapping.csv`; ambiguous offenses are excluded, and unmapped codes fail closed.

The routing service uses a Massachusetts OpenStreetMap PBF extract by default and keeps its generated tiles in a Boston-specific Docker volume. See [Valhalla setup](backend/internal/routing/README.md). The Go Safe Mode planner is connected to a custom Valhalla 3.9.1 A* costing extension that applies risk-zone scores to road edges. Ride endpoints include on-demand routing, route snapshots, and stop-aware route validation.

## Run the Go API

Set a local PostgreSQL password in `.env` (start with the placeholder-only example), start PostgreSQL/PostGIS and Valhalla from the repository root, then run the API from `backend`:

```sh
cp .env.example .env
# Replace the placeholder in .env with a local development password.
set -a
. ./.env
set +a
docker compose up -d db
docker compose --profile routing up -d valhalla
cd backend
export DATABASE_URL="postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/safe_ride?sslmode=disable"
export VALHALLA_URL='http://localhost:8002'
go run ./cmd/server
```

`DATABASE_URL` is required. Keep `.env` local; use a secret manager or deployment environment variables for deployed credentials.

## Open the Streamlit demo

With the Go API running, start the demo client from the repository root:

```sh
python3 -m pip install -r frontend/requirements.txt
API_BASE_URL='http://localhost:8080' python3 -m streamlit run frontend/app.py
```

The rider and driver screens call the existing Go endpoints. See [the frontend README](frontend/README.md) for the demo workflow and its V1 ID-based authorization limitation.
