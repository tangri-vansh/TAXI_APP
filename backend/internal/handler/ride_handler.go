package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"safe-ride/internal/model"
	"safe-ride/internal/service"
)

type RideHandler struct {
	rideService      *service.RideService
	rideRouteService *service.RideRouteService
	rideStopService  *service.RideStopService
}

func NewRideHandler(
	rideService *service.RideService,
	rideRouteService *service.RideRouteService,
	rideStopService *service.RideStopService,
) *RideHandler {
	return &RideHandler{
		rideService: rideService, rideRouteService: rideRouteService, rideStopService: rideStopService,
	}
}

type addRideStopRequest struct {
	RiderID  int64                      `json:"rider_id"`
	Location addRideStopLocationRequest `json:"location"`
}

type addRideStopLocationRequest struct {
	Latitude  *float64 `json:"lat"`
	Longitude *float64 `json:"lng"`
}

type addRideStopResponse struct {
	StopID        int64  `json:"stop_id"`
	RideID        int64  `json:"ride_id"`
	SequenceOrder int    `json:"sequence_order"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}

func (h *RideHandler) AddStop(w http.ResponseWriter, r *http.Request) {
	rideID, err := strconv.ParseInt(r.PathValue("ride_id"), 10, 64)
	if err != nil || rideID <= 0 {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}
	var req addRideStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		req.Location.Latitude == nil || req.Location.Longitude == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_STOP_LOCATION", "Stop location is missing or invalid")
		return
	}
	stop, err := h.rideStopService.AddStop(r.Context(), rideID, req.RiderID, model.Coordinates{
		Latitude: *req.Location.Latitude, Longitude: *req.Location.Longitude,
	})
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(addRideStopResponse{
			StopID: stop.StopID, RideID: stop.RideID, SequenceOrder: stop.SequenceOrder,
			Status: stop.Status, Message: "Stop added successfully",
		})
	case errors.Is(err, service.ErrRideNotFound):
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
	case errors.Is(err, service.ErrRideNotOngoingCannotAddStop):
		writeAPIError(w, http.StatusConflict, "RIDE_NOT_ONGOING_CANNOT_ADD_STOP", "Stops can only be added while the ride is in progress")
	case errors.Is(err, service.ErrNotAuthorizedToAddStop):
		writeAPIError(w, http.StatusForbidden, "NOT_AUTHORIZED_TO_ADD_STOP", "You are not authorized to modify this ride")
	case errors.Is(err, service.ErrInvalidStopLocation):
		writeAPIError(w, http.StatusBadRequest, "INVALID_STOP_LOCATION", "Stop location is missing or invalid")
	case errors.Is(err, service.ErrStopDetourTooLarge):
		writeAPIError(w, http.StatusConflict, "STOP_DETOUR_TOO_LARGE", "Adding this stop exceeds the allowed detour")
	case errors.Is(err, service.ErrStopAddWindowExpired):
		writeAPIError(w, http.StatusConflict, "STOP_ADD_WINDOW_EXPIRED", "Stops cannot be added within 5 minutes of estimated arrival")
	case errors.Is(err, service.ErrDriverLocationUnavailable), errors.Is(err, service.ErrRouteNotYetAvailable):
		writeAPIError(w, http.StatusServiceUnavailable, "ROUTE_NOT_YET_AVAILABLE", "A current road route is not available for this ride")
	case errors.Is(err, service.ErrRideStopsChanged):
		writeAPIError(w, http.StatusConflict, "STOP_ADD_CONFLICT", "The ride route changed; retry the stop request")
	default:
		writeAPIError(w, http.StatusBadGateway, "ROUTING_UNAVAILABLE", "Could not validate the stop route")
	}
}

func (h *RideHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	rideID, err := strconv.ParseInt(r.PathValue("ride_id"), 10, 64)
	if err != nil || rideID <= 0 {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}

	route, err := h.rideRouteService.GetRoute(r.Context(), rideID)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(route)
	case errors.Is(err, service.ErrRideNotFound):
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
	case errors.Is(err, service.ErrRouteNotYetAvailable):
		writeAPIError(w, http.StatusServiceUnavailable, "ROUTE_NOT_YET_AVAILABLE", "Route has not been computed for this ride yet")
	case errors.Is(err, service.ErrDriverLocationUnavailable):
		writeAPIError(w, http.StatusServiceUnavailable, "ROUTE_NOT_YET_AVAILABLE", "A current driver location is not available for this ride")
	default:
		writeAPIError(w, http.StatusBadGateway, "ROUTING_UNAVAILABLE", "Could not calculate a route for this ride")
	}
}

type rideLocationRequest struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type createRideRequest struct {
	RiderID     int64               `json:"user_id"`
	Pickup      rideLocationRequest `json:"pickup"`
	Destination rideLocationRequest `json:"destination"`
	SafeMode    bool                `json:"safe_mode"`
}

type createRideResponse struct {
	RideID  int64  `json:"ride_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type acceptRideRequest struct {
	DriverID int64 `json:"driver_id"`
}

type cancelRideRequest struct {
	RiderID  int64 `json:"rider_id"`
	DriverID int64 `json:"driver_id"`
}

type rideStatusResponse struct {
	RideID  int64  `json:"ride_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (h *RideHandler) CreateRide(w http.ResponseWriter, r *http.Request) {
	var req createRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		req.Pickup.Latitude == nil || req.Pickup.Longitude == nil ||
		req.Destination.Latitude == nil || req.Destination.Longitude == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_LOCATION", "Rider ID, pickup, and destination are required")
		return
	}

	ride, err := h.rideService.CreateRide(
		r.Context(),
		req.RiderID,
		model.Coordinates{Latitude: *req.Pickup.Latitude, Longitude: *req.Pickup.Longitude},
		model.Coordinates{Latitude: *req.Destination.Latitude, Longitude: *req.Destination.Longitude},
		req.SafeMode,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRideInput):
			writeAPIError(w, http.StatusBadRequest, "INVALID_LOCATION", "Pickup and destination must be valid and cannot be the same")
		case errors.Is(err, service.ErrRiderNotFound):
			writeAPIError(w, http.StatusNotFound, "RIDER_NOT_FOUND", "No user exists with this ID")
		case errors.Is(err, service.ErrRiderHasActiveRide):
			writeAPIError(w, http.StatusConflict, "RIDER_HAS_ACTIVE_RIDE", "You already have an active ride in progress")
		default:
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not request ride")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createRideResponse{
		RideID:  ride.RideID,
		Status:  ride.Status,
		Message: "Ride requested successfully, searching for a driver",
	})
}

func (h *RideHandler) GetRide(w http.ResponseWriter, r *http.Request) {
	rideID, err := strconv.ParseInt(r.PathValue("ride_id"), 10, 64)
	if err != nil || rideID <= 0 {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}

	ride, err := h.rideService.GetRide(r.Context(), rideID)
	if errors.Is(err, service.ErrRideNotFound) {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not retrieve ride")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ride)
}

func (h *RideHandler) AcceptRide(w http.ResponseWriter, r *http.Request) {
	rideID, err := strconv.ParseInt(r.PathValue("ride_id"), 10, 64)
	if err != nil || rideID <= 0 {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}

	var req acceptRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DriverID <= 0 {
		writeAPIError(w, http.StatusNotFound, "DRIVER_NOT_FOUND", "No driver exists with this ID")
		return
	}

	err = h.rideService.AcceptRide(r.Context(), rideID, req.DriverID)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rideStatusResponse{
			RideID:  rideID,
			Status:  "ACCEPTED",
			Message: "Ride accepted successfully",
		})
	case errors.Is(err, service.ErrRideNotFound):
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
	case errors.Is(err, service.ErrRideAlreadyAccepted):
		writeAPIError(w, http.StatusConflict, "RIDE_ALREADY_ACCEPTED", "This ride has already been accepted by another driver")
	case errors.Is(err, service.ErrRideAlreadyCompleted):
		writeAPIError(w, http.StatusConflict, "RIDE_ALREADY_COMPLETED", "This ride has already been completed")
	case errors.Is(err, service.ErrRideCancelled):
		writeAPIError(w, http.StatusConflict, "RIDE_CANCELLED", "This ride has been cancelled and cannot be accepted")
	case errors.Is(err, service.ErrDriverNotFound):
		writeAPIError(w, http.StatusNotFound, "DRIVER_NOT_FOUND", "No driver exists with this ID")
	case errors.Is(err, service.ErrDriverNotAvailable):
		writeAPIError(w, http.StatusConflict, "DRIVER_NOT_AVAILABLE", "Driver is not available to accept rides")
	default:
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not accept ride")
	}
}

func (h *RideHandler) StartRide(w http.ResponseWriter, r *http.Request) {
	h.transitionRide(w, r, "start")
}

func (h *RideHandler) CompleteRide(w http.ResponseWriter, r *http.Request) {
	h.transitionRide(w, r, "complete")
}

func (h *RideHandler) CancelRide(w http.ResponseWriter, r *http.Request) {
	rideID, err := strconv.ParseInt(r.PathValue("ride_id"), 10, 64)
	if err != nil || rideID <= 0 {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}

	var req cancelRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "NOT_AUTHORIZED_TO_CANCEL", "Provide the rider ID or assigned driver ID")
		return
	}
	if (req.RiderID <= 0 && req.DriverID <= 0) || (req.RiderID > 0 && req.DriverID > 0) {
		writeAPIError(w, http.StatusBadRequest, "NOT_AUTHORIZED_TO_CANCEL", "Provide exactly one valid rider ID or driver ID")
		return
	}

	err = h.rideService.CancelRide(r.Context(), rideID, req.RiderID, req.DriverID)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rideStatusResponse{
			RideID:  rideID,
			Status:  "CANCELLED",
			Message: "Ride cancelled successfully",
		})
	case errors.Is(err, service.ErrRideNotFound):
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
	case errors.Is(err, service.ErrRideOngoingCannotCancel):
		writeAPIError(w, http.StatusConflict, "RIDE_ONGOING_CANNOT_CANCEL", "A ride in progress cannot be cancelled")
	case errors.Is(err, service.ErrRideAlreadyCompleted):
		writeAPIError(w, http.StatusConflict, "RIDE_ALREADY_COMPLETED", "This ride has already been completed and cannot be cancelled")
	case errors.Is(err, service.ErrRideAlreadyCancelled):
		writeAPIError(w, http.StatusConflict, "RIDE_ALREADY_CANCELLED", "This ride has already been cancelled")
	case errors.Is(err, service.ErrNotAuthorizedToCancel):
		writeAPIError(w, http.StatusForbidden, "NOT_AUTHORIZED_TO_CANCEL", "You are not authorized to cancel this ride")
	default:
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not cancel ride")
	}
}

func (h *RideHandler) transitionRide(w http.ResponseWriter, r *http.Request, action string) {
	rideID, err := strconv.ParseInt(r.PathValue("ride_id"), 10, 64)
	if err != nil || rideID <= 0 {
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
		return
	}

	var req acceptRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DriverID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "DRIVER_NOT_ASSIGNED", "Only the assigned driver can update this ride")
		return
	}

	if action == "start" {
		err = h.rideService.StartRide(r.Context(), rideID, req.DriverID)
	} else {
		err = h.rideService.CompleteRide(r.Context(), rideID, req.DriverID)
	}
	if err != nil {
		h.writeRideTransitionError(w, err, action)
		return
	}

	status, message := "ONGOING", "Trip started successfully"
	if action == "complete" {
		status, message = "COMPLETED", "Trip completed successfully"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rideStatusResponse{
		RideID:  rideID,
		Status:  status,
		Message: message,
	})
}

func (h *RideHandler) writeRideTransitionError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, service.ErrRideNotFound):
		writeAPIError(w, http.StatusNotFound, "RIDE_NOT_FOUND", "No ride exists with this ID")
	case errors.Is(err, service.ErrRideAlreadyCompleted):
		writeAPIError(w, http.StatusConflict, "RIDE_ALREADY_COMPLETED", "This ride has already been completed")
	case errors.Is(err, service.ErrRideCancelled):
		writeAPIError(w, http.StatusConflict, "RIDE_CANCELLED", "This ride has been cancelled and cannot be updated")
	case errors.Is(err, service.ErrDriverNotAssigned):
		writeAPIError(w, http.StatusForbidden, "DRIVER_NOT_ASSIGNED", "Only the driver assigned to this ride can update it")
	case errors.Is(err, service.ErrRideNotAcceptedCannotStart):
		writeAPIError(w, http.StatusConflict, "RIDE_NOT_ACCEPTED_CANNOT_START", "Ride must be accepted before it can be started")
	case errors.Is(err, service.ErrRideAlreadyOngoing):
		writeAPIError(w, http.StatusConflict, "RIDE_ALREADY_ONGOING", "This ride has already started")
	case errors.Is(err, service.ErrRideNotOngoingCannotComplete):
		writeAPIError(w, http.StatusConflict, "RIDE_NOT_ONGOING_CANNOT_COMPLETE", "Ride must be ongoing before it can be completed")
	default:
		message := "Could not start ride"
		if action == "complete" {
			message = "Could not complete ride"
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", message)
	}
}
