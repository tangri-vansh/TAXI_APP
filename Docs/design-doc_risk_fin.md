# Safety-Aware Ride-Hailing App — Design Reference

**Status:** Design phase complete (Week 1) — API specs, ride state machine, and database schema finalized.
**Timeline:** Sep 14 – Oct 25, 2026

---

## 1. Project Overview

A ride-hailing app (Uber/Ola-inspired) differentiated by **Safe Mode**: historical crime data is aggregated into geographic risk zones, and the routing system can factor that into route selection.

> Safe Mode never claims a route is "safe" — only that it has **lower historical reported-incident risk**.

**Stack:** React (frontend) · Go (backend) · PostgreSQL + PostGIS · external routing engine · Redis (later, for fast-changing data) · Kafka (later, only if justified) · Docker.

**Architecture style:** Modular monolith — not microservices.

**Explicitly out of V1 scope:** payments, ratings, coupons, driver earnings, complex authentication, ML crime prediction, Kubernetes, microservices, Kafka without a real need, an overbuilt frontend.

---

## 2. Architecture

```
React Frontend
      ↓
Go Backend / REST APIs
      ↓
PostgreSQL + PostGIS
      ↓
Routing Engine
```

Planned later:

```
                 ┌── Redis (fast-changing data, e.g. driver location)
                 │
React → Go Backend ── PostgreSQL/PostGIS
                 │
                 ├── Routing Engine
                 │
                 └── Safety/Risk processing
```

**Crime data pipeline:**

```
Raw Crime Data → Clean/Process → Geographical Aggregation → Risk Zones → Risk Score → Route Optimizer
```

Currently using zone-level risk (not road-segment-level).

---

## 3. Database Schema

### Entities

| Entity | Purpose |
|---|---|
| `USER` | Riders |
| `DRIVER` | Drivers (linked to a `USER`) |
| `RIDE` | A single ride request/trip |
| `RIDE_STOP` | Optional mid-ride stops on a ride |
| `DRIVER_LOCATION` | Historical log of driver positions |
| `ROUTE` | Computed route for a ride |
| `CRIME_INCIDENT` | Raw crime data points |
| `RISK_ZONE` | Aggregated risk area, derived from incidents |

### Relationships

```
USER            1 ─── N   RIDE
DRIVER          1 ─── N   RIDE
DRIVER          1 ─── N   DRIVER_LOCATION
RIDE            1 ─── N   ROUTE
RIDE            1 ─── N   RIDE_STOP
CRIME_INCIDENT  N ─── 1   RISK_ZONE
```

### Table definitions

**USER**
| Column | Notes |
|---|---|
| `user_id` (PK) | |
| `name` | |
| `phone` | |
| `password_hash` | Never returned in any API response; hashed (e.g. bcrypt) |

**DRIVER**
| Column | Notes |
|---|---|
| `driver_id` (PK) | |
| `user_id` (FK) | |
| `status` | `AVAILABLE` / `BUSY` |
| `vehicle_type` | Kept as columns on `DRIVER` (not a separate table) — a driver has exactly one active vehicle in V1 |
| `plate_number` | Must be unique |

**RIDE**
| Column | Notes |
|---|---|
| `ride_id` (PK) | |
| `rider_id` (FK → USER) | |
| `driver_id` (FK → DRIVER, nullable until accepted) | |
| `pickup` | |
| `destination` | |
| `status` | `REQUESTED` / `ACCEPTED` / `ONGOING` / `COMPLETED` / `CANCELLED` |

**RIDE_STOP**
| Column | Notes |
|---|---|
| `stop_id` (PK) | |
| `ride_id` (FK) | |
| `location` | |
| `sequence_order` | Server-assigned, not client-supplied |
| `status` | `PENDING` / `COMPLETED` |

A ride with no stops simply has zero `RIDE_STOP` rows — no special-casing needed; the 1-to-N relationship handles cardinality zero naturally.

**DRIVER_LOCATION**
| Column | Notes |
|---|---|
| `location_id` (PK) | |
| `driver_id` (FK) | |
| `location` | |
| `timestamp` | |

**Decision:** insert-only (one new row per ping), not upsert. A driver's client pings every **30–45 seconds**, keeping row growth manageable while preserving full movement history (useful for Safe Mode route verification, disputes, analytics later). Requires a composite index on `(driver_id, timestamp DESC)` for fast "current location" lookups. Redis will later hold *current* location for speed; Postgres keeps the historical log.

**ROUTE**
| Column | Notes |
|---|---|
| `route_id` (PK) | |
| `ride_id` (FK) | |
| `geometry` | JSONB array of Valhalla route-leg polylines and per-leg distance/ETA; includes pending `RIDE_STOP` waypoints |
| `distance_km`, `eta_seconds` | Total road-route distance and travel time |
| `risk_score` | Safe Mode risk exposure; NULL for Normal Mode |
| `mode`, `safe_mode_used`, `fallback`, `fallback_reason` | Records the route-selection result |
| `normal_duration_seconds`, `selected_beta`, `created_at` | Safe Mode comparison and snapshot metadata |

**CRIME_INCIDENT**
| Column | Notes |
|---|---|
| `crime_incident_id` (PK) | |
| `source_name`, `source_incident_number`, `offense_code` | Together identify a source incident offense for idempotent imports |
| `offense_description`, `risk_category`, `risk_weight` | Classification is from the deterministic Boston code mapping; excluded offenses are not stored |
| `occurred_at`, `location` | `location` is a PostGIS geography point and is required for spatial scoring |
| `source_snapshot_date` | Snapshot date used for the import |
| `risk_zone_id` (nullable FK → RISK_ZONE) | Assigned when the risk-zone build runs |

**RISK_ZONE**
| Column | Notes |
|---|---|
| `risk_zone_id` (PK) | |
| `source_name` | Incident source whose records contributed to this zone |
| `grid_size_m`, `grid_center_x_m`, `grid_center_y_m` | Identifies the snapped square cell in UTM Zone 19N (EPSG:32619) |
| `zone_geom` | 500m × 500m projected polygon; GiST indexed |
| `incident_count`, `raw_risk`, `damped_risk`, `normalized_risk` | Per-cell count and design-formula score components |
| `recency_half_life_days`, `scored_at` | Settings/reference time used for this scoring build |

The risk-zone build replaces the current grid and score set atomically. Only populated cells are stored; missing cells mean risk data is unavailable, not a zero-risk observation. Historical score builds are not retained in V1.

---

## 4. Ride State Machine

### States
`REQUESTED` → `ACCEPTED` → `ONGOING` → `COMPLETED`
`REQUESTED` → `CANCELLED`
`ACCEPTED` → `CANCELLED`

`COMPLETED` and `CANCELLED` are terminal — no further transitions are possible from either.

### Transition Table

| Current State | accept | start | complete | cancel | add-stop |
|---|---|---|---|---|---|
| **REQUESTED** | ✅ → ACCEPTED | ❌ | ❌ | ✅ → CANCELLED | ❌ (use `POST /rides` request body instead) |
| **ACCEPTED** | ❌ | ✅ → ONGOING | ❌ | ✅ → CANCELLED | ❌ |
| **ONGOING** | ❌ | ❌ | ✅ → COMPLETED | ❌ | ✅ (stays ONGOING) |
| **COMPLETED** | ❌ | ❌ | ❌ | ❌ | ❌ |
| **CANCELLED** | ❌ | ❌ | ❌ | ❌ | ❌ |

### Rules
- A completed ride cannot be cancelled; a cancelled ride cannot be accepted.
- Two drivers cannot simultaneously accept the same ride.
- Cancellation before acceptance (`REQUESTED`) is rider-only. After acceptance (`ACCEPTED`), either rider or driver may cancel.
- Once `ONGOING`, the ride cannot be cancelled — no abort/emergency-stop flow in V1.
- Rides/records are never deleted — status changes, but history is retained for audit.
- The other participant is notified on cancellation.

### Deferred: ride expiry / no-driver-available
Intentionally **not implemented yet**. A `REQUESTED` ride currently waits indefinitely for a driver to accept or the rider to cancel. Auto-expiry (e.g. cancel after N seconds with no driver) is deferred until WebSockets are introduced, since the timeout mechanism and the real-time notification mechanism are being built together rather than as separate throwaway logic now.

---

## 5. API Specification

Format: `HTTP METHOD + ENDPOINT`, Purpose, Request, Response, Who, Preconditions, DB Changes, Errors.

### POST /users
**Purpose:** Register a new user.

Request:
```json
{ "name": "vansh tangri", "phone": "9999999999", "password": "123@456" }
```
Response:
```json
{ "user_id": 101, "message": "User registered successfully" }
```
Errors:
```json
{ "error": "VALIDATION_ERROR", "message": "Name and password are required" }
{ "error": "USER_ALREADY_EXISTS", "message": "A user with this phone number already exists" }
{ "error": "WEAK_PASSWORD", "message": "Password does not meet minimum requirements" }
```
Note: `password_hash` is stored server-side and never returned in any response.

---

### POST /drivers
**Purpose:** Register a new driver (linked to an existing user).

Errors:
```json
{ "error": "VALIDATION_ERROR", "message": "Name, vehicle type, and plate number are required" }
{ "error": "USER_NOT_FOUND", "message": "No user exists with this ID to link as a driver" }
{ "error": "DRIVER_ALREADY_EXISTS", "message": "This user is already registered as a driver" }
{ "error": "PLATE_NUMBER_ALREADY_REGISTERED", "message": "This vehicle plate number is already in use" }
```

---

### POST /rides
**Purpose:** Request a new ride.

Response:
```json
{ "ride_id": 501, "status": "REQUESTED", "message": "Ride requested successfully, searching for a driver" }
```
Errors:
```json
{ "error": "RIDER_NOT_FOUND", "message": "No user exists with this ID" }
{ "error": "INVALID_LOCATION", "message": "Pickup and destination cannot be the same" }
{ "error": "RIDER_HAS_ACTIVE_RIDE", "message": "You already have an active ride in progress" }
```

---

### POST /rides/{ride_id}/accept
**Who:** Driver

**Preconditions:** Ride exists · status = `REQUESTED` · driver exists · driver is `AVAILABLE`

**Changes:** `ride.driver_id` set · `ride.status` = `ACCEPTED` · `driver.status` = `BUSY` · notify rider

Errors:
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "RIDE_ALREADY_ACCEPTED", "message": "This ride has already been accepted by another driver" }
{ "error": "RIDE_ALREADY_COMPLETED", "message": "This ride has already been completed" }
{ "error": "RIDE_CANCELLED", "message": "This ride has been cancelled and cannot be accepted" }
{ "error": "DRIVER_NOT_FOUND", "message": "No driver exists with this ID" }
{ "error": "DRIVER_NOT_AVAILABLE", "message": "Driver is not available to accept rides" }
```

---

### POST /rides/{ride_id}/start
**Who:** The driver assigned to this ride

**Preconditions:** Ride exists · status = `ACCEPTED` · caller is the assigned driver (Option A auth, see §6)

**Changes:** `ride.status` = `ONGOING`

Response:
```json
{ "ride_id": 501, "status": "ONGOING", "message": "Trip started successfully" }
```
Errors:
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "RIDE_NOT_ACCEPTED_CANNOT_START", "message": "Ride must be accepted before it can be started" }
{ "error": "RIDE_ALREADY_ONGOING", "message": "This ride has already started" }
{ "error": "RIDE_ALREADY_COMPLETED", "message": "This ride has already been completed" }
{ "error": "RIDE_CANCELLED", "message": "This ride has been cancelled and cannot be started" }
{ "error": "DRIVER_NOT_ASSIGNED", "message": "Only the driver assigned to this ride can start it" }
```

---

### POST /rides/{ride_id}/complete
**Who:** The driver assigned to this ride

**Preconditions:** Ride exists · status = `ONGOING` · caller is the assigned driver

**Changes:** `ride.status` = `COMPLETED`

Response:
```json
{ "ride_id": 501, "status": "COMPLETED", "message": "Trip completed successfully" }
```
Errors:
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "RIDE_NOT_ONGOING_CANNOT_COMPLETE", "message": "Ride must be ongoing before it can be completed" }
{ "error": "RIDE_ALREADY_COMPLETED", "message": "This ride has already been completed" }
{ "error": "RIDE_CANCELLED", "message": "This ride has been cancelled and cannot be completed" }
{ "error": "DRIVER_NOT_ASSIGNED", "message": "Only the driver assigned to this ride can complete it" }
```

---

### POST /rides/{ride_id}/cancel
**Who:** Rider (if `REQUESTED`) · rider or driver (if `ACCEPTED`)

**Preconditions:** Ride exists · status is `REQUESTED` or `ACCEPTED` · caller is a valid participant

**Changes:** `ride.status` = `CANCELLED` · notify the other participant

Errors:
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "RIDE_ONGOING_CANNOT_CANCEL", "message": "A ride in progress cannot be cancelled" }
{ "error": "RIDE_ALREADY_COMPLETED", "message": "This ride has already been completed and cannot be cancelled" }
{ "error": "RIDE_ALREADY_CANCELLED", "message": "This ride has already been cancelled" }
{ "error": "NOT_AUTHORIZED_TO_CANCEL", "message": "You are not authorized to cancel this ride" }
```

---

### POST /rides/{ride_id}/stops
**Purpose:** Add a single stop to an ongoing ride (one stop per call).

**Who:** The rider on this ride

**Preconditions:**
- Ride exists · status = `ONGOING`
- Caller is authorized on this ride
- Location is valid
- New stop's road-route detour does not exceed **20% distance** or **10 minutes** added ETA; exceeding either limit rejects the stop
- Time remaining to destination ≥ **5 minutes**

**Changes:** Insert `RIDE_STOP` row (`sequence_order` server-assigned) · recompute the road route geometry/distance/ETA · notify driver when WebSocket notifications are implemented

Request:
```json
{ "rider_id": 21, "location": { "lat": 42.35, "lng": -71.06 } }
```
Response:
```json
{ "stop_id": 12, "ride_id": 501, "sequence_order": 1, "status": "PENDING", "message": "Stop added successfully" }
```
Errors:
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "RIDE_NOT_ONGOING_CANNOT_ADD_STOP", "message": "Stops can only be added while the ride is in progress" }
{ "error": "NOT_AUTHORIZED_TO_ADD_STOP", "message": "You are not authorized to modify this ride" }
{ "error": "INVALID_STOP_LOCATION", "message": "Stop location is missing or invalid" }
{ "error": "STOP_DETOUR_TOO_LARGE", "message": "Adding this stop increases the route by more than 20% or 10 minutes" }
{ "error": "STOP_ADD_WINDOW_EXPIRED", "message": "Stops cannot be added within 5 minutes of estimated arrival" }
{ "error": "ROUTE_NOT_YET_AVAILABLE", "message": "A current road route is not available for this ride" }
{ "error": "STOP_ADD_CONFLICT", "message": "The ride route changed; retry the stop request" }
```
Thresholds are hardcoded as named constants for now (`MaxStopDetourRatio = 0.20`, `MaxStopAddedDurationSecs = 600`, `MinTimeBeforeDropoffSecs = 300`) — tuning with real data is a later concern.

The route comparison starts at the assigned driver's latest recorded location, follows existing pending stops in server-assigned order, and ends at the ride destination. The candidate route inserts the new stop before the destination. Route distance and ETA come from Valhalla road routing; straight-line distance is not used. The server rechecks the ride state, rider, and pending-stop count inside a transaction before inserting the stop, and commits its recalculated route snapshot in that same transaction.

---

### POST /drivers/{driver_id}/location
**Purpose:** Log a new driver location ping (insert-only, every 30–45s).

For the assigned driver on an ongoing ride, a ping within 50 meters of the next pending stop marks that stop `COMPLETED`. Only one stop is completed per ping, in sequence order, and the location insert and stop update share a transaction. The success response includes `completed_stop_id` when this happens.

Errors:
```json
{ "error": "DRIVER_NOT_FOUND", "message": "No driver exists with this ID" }
{ "error": "INVALID_LOCATION", "message": "Location coordinates are missing or invalid" }
```
No state/conflict checks needed — insert-only, existence + validity are the only checks.

---

### GET /drivers/available
**Purpose:** Find available drivers near the rider's current location.

**Requires:** `lat`, `long` (rider's current position) as query params — e.g. `GET /drivers/available?lat={lat}&long={long}`

**Search radius is hardcoded server-side** (`SEARCH_RADIUS_KM = 2`), same pattern as the detour/time constants used for `add-stop` — not client-supplied, since it's a system tuning parameter, not user data. If zero drivers fall within the radius, the response is simply an empty list — no auto-expansion of the radius in V1; that kind of fallback strategy belongs with the broader no-driver-available handling already deferred to the WebSocket implementation (§4).

Errors:
```json
{ "error": "VALIDATION_ERROR", "message": "Latitude and longitude are required" }
```
An empty result (`{ "drivers": [] }`) is a **valid response**, not an error.

---

### GET /rides/{ride_id}
Read-only; valid regardless of ride state (e.g. checking a `CANCELLED` ride's status is normal). `ride_id` is part of the URL, not the request body.

Errors:
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
```

### GET /rides/{ride_id}/route
Returns the latest saved route snapshot. If no route exists yet, it calculates a route from the ride's pickup, destination, and saved Safe Mode preference, saves the snapshot, and returns it. For an ongoing ride, each request refreshes the route from the driver's latest recorded location through pending stops in order, then persists that snapshot. It returns Valhalla's encoded road geometry per leg, distance, ETA, and Safe Mode/fallback metadata. The default Safe Mode beta schedule is `120, 60, 30, 15, 7.5`; beta is the per-edge penalty coefficient in seconds for a normalized risk score in [0,1]. This schedule is an initial implementation setting and should be calibrated during route-level evaluation.

Example successful response:
```json
{
  "ride_id": 42,
  "mode": "SAFE_MODE",
  "distance_km": 4.1,
  "duration_seconds": 510,
  "legs": [
    { "distance_km": 4.1, "duration_seconds": 510, "shape": "encoded_polyline" }
  ],
  "safe_mode_used": true,
  "fallback": false,
  "risk_exposure": 8.25,
  "normal_duration_seconds": 495,
  "selected_beta": 30
}
```

Each calculated route is persisted as a snapshot in `routes`. Adding an accepted stop saves its recalculated route in the same transaction as the stop. For an ongoing ride, route calculation uses the driver's latest recorded location and pending stops in sequence order.
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "ROUTE_NOT_YET_AVAILABLE", "message": "Route has not been computed for this ride yet" }
{ "error": "ROUTING_UNAVAILABLE", "message": "Could not calculate a route for this ride" }
```

---

## 6. Cross-Cutting Decisions

**Authentication/Authorization (Option A, V1):**
Caller-supplied `rider_id` / `driver_id` is compared against the ride's stored IDs — logical authorization without real authentication (no tokens/sessions/passwords in the request flow). Passwords are still hashed and stored safely; "complex authentication" being out of V1 scope means no session/token infrastructure, not unsafe credential handling.
> **Planned V2:** proper authentication (tokens/sessions) replacing the ID-comparison approach.

**Ride expiry / no-driver-available:**
Deferred until WebSockets are implemented (see §4). `REQUESTED` rides currently wait indefinitely.

**Driver location strategy:**
Insert-only, 30–45s ping interval, indexed on `(driver_id, timestamp DESC)`. Redis will later serve *current* location for speed; Postgres retains the full history.

**Vehicle info:**
Stored as columns directly on `DRIVER` (not a separate `VEHICLE` table) — a driver has exactly one active vehicle in V1 scope.

---

## 7. Safe Mode Design

### A. What is a risk zone?
A fixed grid cell, **500m × 500m** in Boston's UTM Zone 19N projection (EPSG:32619). Validation on the 85,921 mapped incidents with usable coordinates found 503 populated 500m cells (mean 170.8 incidents/cell, median 53, 90th percentile 344, and 4.8% singleton cells). This is sufficiently dense for V1; retain 500m rather than widening to 750m–1km. Revalidate if the dataset or supported region changes materially.

### Data source
V1 uses the Boston Police Department crime incident CSV currently stored as `tmpivat4nah.csv`. The inspected snapshot contains 305,787 rows dated 2023-01-01 through 2026-10-19, with 123 distinct `OFFENSE_DESCRIPTION` values. `Lat` and `Long` are usable for 292,083 rows. Incidents without valid coordinates and offenses excluded by the mapping are not inserted into the spatial incident table; their counts are recorded by the source inventory and import output. The source dataset is updated over time and may be preliminary or revised; record the source and import snapshot date with each data load.

This is reported police incident data, not a complete measure of crime or personal safety. Coordinate coverage and reporting practices can vary by incident type. Do not imply that a mapped point is an exact event address.

**Fields used:** `INCIDENT_NUMBER` (source identity/deduplication analysis), `OFFENSE_CODE`, `OFFENSE_DESCRIPTION` (classification input), `OCCURRED_ON_DATE` (recency), `Lat`, and `Long`. The local CSV's `OFFENSE_CODE_GROUP` and `UCR_PART` fields are empty, so they are not classification sources. A deterministic code-based mapping is documented in `Docs/boston_offense_mapping.md` and `backend/data/boston_offense_mapping.csv`; excluded and ambiguous offenses are not imported or scored. Do not infer categories from free text at request time. Unmapped offenses must not silently receive a risk weight.

The established weights are retained through the Boston categories: `violent_person_offense` and `robbery` = 1.0; `harassment_threats` = 0.8; `theft` and `vehicle_theft` = 0.5; `burglary` and `property_damage` = 0.3. Fraud and offenses outside the approved physical-safety taxonomy are excluded. Do not use offense labels as an unverified severity multiplier.

The initial supported Safe Mode region is the Boston area covered by both the incident data and the configured road graph. Outside that coverage, the system must not present a Boston-derived risk score as applicable; define the API/UI behavior as unsupported coverage or unavailable Safe Mode before release.

### B. Risk score formula

```
RawRisk(zone)    = Σ_i [w_type(type_i) × recency_weight(i)]     for incidents i in zone, type_i ≠ fraud
DampedRisk(zone) = log(1 + RawRisk(zone))
NormRisk(zone)   = (DampedRisk(zone) − min_all_zones) / (max_all_zones − min_all_zones)     → range [0, 1]
```

- `w_type`: `violent_person_offense = 1.0`, `robbery = 1.0`, `harassment_threats = 0.8`, `theft = 0.5`, `vehicle_theft = 0.5`, `burglary = 0.3`, `property_damage = 0.3`; excluded categories have no score
- `recency_weight(i; H) = 0.5 ^ (days_since_incident_i / H)`, where `H` is the centrally configured recency half-life in days, default `365` (one year). An incident's weight halves every `H` days.
- Keep `H` fixed for each risk-score build, never vary it by rider or route, and record it with the build for reproducibility. It may be recalibrated after evaluation of the Boston data.
- Log-damping + min-max normalization prevent a small number of extreme-frequency zones from dominating the score and keep `NormRisk` on a predictable [0,1] scale, making `β` in the cost function tunable and interpretable

This is a simplified version of established hotspot-mapping methods (kernel density estimation, and specifically ProMap's decay-weighted incident summation) — not an arbitrary invented formula.

### C. Zone → edge mapping
```
Risk(edge) = NormRisk(zone containing edge's midpoint)
```
No separate proximity buffer — the grid cell itself already defines "nearby." The current build stores populated incident cells only; an absent cell means risk data is unavailable there, not that its risk is zero. Safe Mode route integration must preserve this distinction.

### D. Routing cost function
Only active when Safe Mode is explicitly selected; Normal Mode is unaffected.

```
Normal mode: cost(edge) = Time(edge)
Safe mode:   cost(edge) = Time(edge) + β × Risk(edge)
```

### Routing algorithm
**V1: A\***, run with a different cost function per mode. No K-shortest paths, Pareto filtering, or Contraction Hierarchies in V1 — explicitly deferred to V2 if the basic system works.

### Enforcing the "safest route ≤ 20% longer" cap
A* alone doesn't guarantee this — a bounded search loop around it is needed:
1. Run A* with `cost = Time` → `T_min` (normal-mode baseline)
2. Run A* with `cost = Time + β·Risk` (starting β) → `T_safe`
3. If `T_safe ≤ 1.2 × T_min` → accept, return as the Safe Mode route
4. Else → reduce β and rerun (bounded number of iterations — cheap, since routes are small per-trip graphs)
5. If even a small β still exceeds the cap → **fall back to the normal-mode route** and inform the rider no safer alternative was found within the acceptable detour, rather than silently ignoring the cap

### User-facing behavior
- Rider selects Safe Mode explicitly; Normal Mode is the default and unaffected by any of this logic.
- If a safer route is found within the 20% cap, it's shown as the Safe Mode route.
- If no route satisfies the cap, fall back to the normal/fastest route with an explicit message that no safer alternative was found within the acceptable detour.
- Safe Mode never claims a route is "safe" — only that it has lower historical reported-incident risk (per §1).

### Evaluation
1. Pick 3–5 fixed origin–destination pairs where the shortest path is known to pass near a high-risk grid cell.
2. For each pair, compute both Normal and Safe routes, recording:
   - `T_normal`, `T_safe` (travel time)
   - `RiskExposure_normal`, `RiskExposure_safe` (sum of `Risk(edge)` along the route)
3. Report as a before/after table: % time increase vs. % risk-exposure decrease per route pair. This is the concrete evidence that Safe Mode does something, for the README/demo.

---

## 8. Status Summary

| Area | Status |
|---|---|
| Architecture | ✅ Done |
| ER / database schema | ✅ Done |
| Ride state machine + transition table | ✅ Done |
| RIDE endpoint specs + errors | ✅ Done |
| USER / DRIVER / DRIVER_LOCATION endpoint specs + errors | ✅ Done |
| Auth approach | ✅ Decided (Option A, V1) |
| Ride expiry / no-driver-available | 🕓 Deferred to WebSocket implementation |
| Safe Mode (risk zones, scoring, route selection) | 🟡 Boston mapping/import, grid validation, scoring, Go midpoint lookup/cost calculation, bounded retry/20% detour/fallback, Valhalla edge-cost integration, stop-aware routing, route snapshots, and an initial five-pair evaluation implemented; measured risk-exposure reductions are modest and should be improved with targeted route-pair analysis |
| Go backend implementation | 🟡 Core user, driver, ride lifecycle, location, nearby-driver, routing, ride-stop, and route-persistence endpoints implemented; automated Go tests pass |

---

## 9. Timeline

| Week | Dates | Focus |
|---|---|---|
| 1 | Sep 14–20 | API specs, state machine, DB schema *(this document)* |
| 2 | Sep 21–27 | Go project setup, Postgres/PostGIS, Users/Drivers/Rides |
| 3 | Sep 28–Oct 4 | Core ride flow: request, find drivers, accept, cancel, complete |
| 4 | Oct 5–11 | Driver location, routing engine, ETA, route handling |
| 5 | Oct 12–18 | Safe Mode: crime data processing, risk zones, risk scoring |
| 6 | Oct 19–25 | Frontend integration, testing, Docker, deployment, docs |
