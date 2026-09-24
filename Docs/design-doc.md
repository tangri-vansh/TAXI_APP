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
| `geometry` | Must account for 0+ waypoints from `RIDE_STOP`, not just pickup→destination |
| `distance` | |
| `eta` | |
| `risk_score` | Used by Safe Mode |

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
- New stop's detour ≤ **20% distance** or **10 minutes** added ETA (whichever trips first)
- Time remaining to destination ≥ **5 minutes**

**Changes:** Insert `RIDE_STOP` row (`sequence_order` server-assigned) · recompute `ROUTE.geometry/distance/eta` · notify driver

Request:
```json
{ "location": { "lat": 15.5943, "lng": 73.8142 } }
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
```
Thresholds are hardcoded as named constants for now (`MAX_DETOUR_PERCENT = 20`, `MAX_DETOUR_MINUTES = 10`, `MIN_TIME_BEFORE_DROPOFF_MINUTES = 5`) — tuning with real data is a later concern.

---

### POST /drivers/{driver_id}/location
**Purpose:** Log a new driver location ping (insert-only, every 30–45s).

Errors:
```json
{ "error": "DRIVER_NOT_FOUND", "message": "No driver exists with this ID" }
{ "error": "INVALID_LOCATION", "message": "Location coordinates are missing or invalid" }
```
No state/conflict checks needed — insert-only, existence + validity are the only checks.

---

### GET /drivers/available
**Purpose:** Find available drivers near a location.

**Requires:** latitude, longitude, radius (not yet reflected in the original endpoint list — needs to be added as query params)

Errors:
```json
{ "error": "VALIDATION_ERROR", "message": "Latitude, longitude, and radius are required" }
{ "error": "INVALID_RADIUS", "message": "Radius must be a positive number within allowed limits" }
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
Read-only.
```json
{ "error": "RIDE_NOT_FOUND", "message": "No ride exists with this ID" }
{ "error": "ROUTE_NOT_YET_AVAILABLE", "message": "Route has not been computed for this ride yet" }
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

## 7. Status Summary

| Area | Status |
|---|---|
| Architecture | ✅ Done |
| ER / database schema | ✅ Done |
| Ride state machine + transition table | ✅ Done |
| RIDE endpoint specs + errors | ✅ Done |
| USER / DRIVER / DRIVER_LOCATION endpoint specs + errors | ✅ Done |
| Auth approach | ✅ Decided (Option A, V1) |
| Ride expiry / no-driver-available | 🕓 Deferred to WebSocket implementation |
| Safe Mode (risk zones, scoring, route selection) | ⬜ Not started — Week 5 |
| Go backend implementation | ⬜ Not started — Week 2 |

---

## 8. Timeline

| Week | Dates | Focus |
|---|---|---|
| 1 | Sep 14–20 | API specs, state machine, DB schema *(this document)* |
| 2 | Sep 21–27 | Go project setup, Postgres/PostGIS, Users/Drivers/Rides |
| 3 | Sep 28–Oct 4 | Core ride flow: request, find drivers, accept, cancel, complete |
| 4 | Oct 5–11 | Driver location, routing engine, ETA, route handling |
| 5 | Oct 12–18 | Safe Mode: crime data processing, risk zones, risk scoring |
| 6 | Oct 19–25 | Frontend integration, testing, Docker, deployment, docs |
