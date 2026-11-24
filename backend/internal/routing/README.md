# Valhalla routing integration

The `routing` Compose profile runs Valhalla on port `8002`. The V1 pilot region is Boston. Compose defaults to the Geofabrik Massachusetts OpenStreetMap PBF extract, which covers Boston and nearby roads. Crime incidents provide risk scores; OpenStreetMap provides the road graph, turn restrictions, and route geometry.

Start Valhalla with the Boston graph:

```sh
docker compose --profile routing up -d valhalla
```

To pin or override the extract, set `VALHALLA_TILE_URL` to a Boston-area PBF URL. Valhalla stores the source and generated graph under the new `valhalla_boston_files` volume. This separate volume prevents the existing Delhi tiles in `valhalla_custom_files` from being reused or deleted. The first startup downloads the Massachusetts extract and builds graph tiles, which can take several minutes. The PBF is an OpenStreetMap-derived dataset; retain the attribution and ODbL notice when distributing map-derived data.

The Go adapter is in `valhalla.go`. Configure its base URL as `http://localhost:8002` when the Go server runs on the host, or `http://valhalla:8002` when it runs in the Compose network.

`safe_mode.go` implements the Go-side route-selection policy: calculate a normal baseline, try up to five descending beta values, enforce the 20% ETA cap, and return the normal route with a fallback reason if no lower-risk candidate qualifies. `safe_mode_valhalla.go` implements `SafeModeEngine` and exposes `Plan`, which connects that planner to the Valhalla client and risk-zone service.

`valhalla-safe-mode.patch` extends Valhalla 3.9.1 `auto` costing with risk-zone polygons and a beta. For each base graph edge, the custom cost uses its road-geometry midpoint to find the containing populated risk zone, then adds `beta * normalized_risk` to elapsed-time cost during A*. Hierarchy pruning is disabled for these requests so shortcut edges cannot hide constituent edge risk. Edges without zone coverage are unavailable to Safe Mode; unknown risk is never treated as zero. `valhalla.Dockerfile` builds the patched engine, and Compose uses that image.

For each route evaluation, the Go adapter requests Valhalla trace attributes, derives route-edge midpoint coordinates, and bulk-loads their zone risk for the planner's risk-exposure and coverage checks. The planner runs a standard-time baseline, then beta attempts, and accepts only a fully covered route with lower risk exposure and no more than 20% additional ETA. Otherwise it returns the baseline with a fallback reason.

`GET /rides/{ride_id}/route` now loads the ride's saved Safe Mode preference and calculates the route on demand. Normal Mode calls the ordinary Valhalla route API. Safe Mode runs the normal baseline plus a descending beta schedule (`120, 60, 30, 15, 7.5`) through the planner; it returns either a qualifying lower-risk route or the normal route with an explicit fallback reason. The response includes encoded Valhalla leg shapes, distance, ETA, whether Safe Mode was used, and risk/fallback metadata. Set `VALHALLA_URL` when the Go server cannot reach Valhalla at `http://localhost:8002`.

`GET /rides/{ride_id}/route` returns the latest persisted snapshot, or calculates and stores one when the ride has no route yet. The `routes` table stores per-leg encoded geometry, total distance and ETA, mode, and Safe Mode/fallback metadata. `POST /rides/{ride_id}/stops` validates a proposed stop by comparing Valhalla routes from the driver's latest recorded location through current pending stops, with and without the proposed stop. It commits the new stop and recalculated route snapshot together. Build the custom image before running Safe Mode with `docker compose --profile routing build valhalla`. A normal `auto` route is still Normal Mode and must not be presented as Safe Mode.

Driver location pings complete only the next pending stop when the assigned driver is within 50 meters. This is handled transactionally with the ping. See `backend/cmd/evaluate-safe-mode/README.md` for the five-pair Safe Mode route evaluation command.
