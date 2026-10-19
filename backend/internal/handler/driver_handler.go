package handler

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"

	"safe-ride/internal/model"
	"safe-ride/internal/service"
)

type DriverHandler struct {
	driverService         *service.DriverService
	driverLocationService *service.DriverLocationService
}

func NewDriverHandler(
	driverService *service.DriverService,
	driverLocationService *service.DriverLocationService,
) *DriverHandler {
	return &DriverHandler{
		driverService:         driverService,
		driverLocationService: driverLocationService,
	}
}

type createDriverRequest struct {
	UserID      int64  `json:"user_id"`
	VehicleType string `json:"vehicle_type"`
	PlateNumber string `json:"plate_number"`
}

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type createDriverResponse struct {
	DriverID int64  `json:"driver_id"`
	Message  string `json:"message"`
}

type driverLocationRequest struct {
	Latitude  *float64 `json:"lat"`
	Longitude *float64 `json:"lng"`
}

type messageResponse struct {
	Message string `json:"message"`
}

type availableDriversResponse struct {
	Drivers []model.AvailableDriver `json:"drivers"`
}

func (h *DriverHandler) CreateDriver(w http.ResponseWriter, r *http.Request) {
	var req createDriverRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Request body must be valid JSON")
		return
	}

	driver, err := h.driverService.CreateDriver(
		r.Context(), req.UserID, req.VehicleType, req.PlateNumber,
	)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidDriverInput):
			writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "User ID, vehicle type, and plate number are required")
		case errors.Is(err, service.ErrDriverUserNotFound):
			writeAPIError(w, http.StatusNotFound, "USER_NOT_FOUND", "No user exists with this ID to link as a driver")
		case errors.Is(err, service.ErrDriverAlreadyRegistered):
			writeAPIError(w, http.StatusConflict, "DRIVER_ALREADY_EXISTS", "This user is already registered as a driver")
		case errors.Is(err, service.ErrPlateAlreadyRegistered):
			writeAPIError(w, http.StatusConflict, "PLATE_NUMBER_ALREADY_REGISTERED", "This vehicle plate number is already in use")
		default:
			writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not register driver")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createDriverResponse{
		DriverID: driver.DriverID,
		Message:  "Driver registered successfully",
	})
}

func (h *DriverHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	driverID, err := strconv.ParseInt(r.PathValue("driver_id"), 10, 64)
	if err != nil || driverID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_LOCATION", "Driver ID and location coordinates must be valid")
		return
	}

	var req driverLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Latitude == nil || req.Longitude == nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_LOCATION", "Location coordinates are missing or invalid")
		return
	}

	err = h.driverLocationService.CreateLocation(
		r.Context(), driverID, *req.Latitude, *req.Longitude,
	)
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(messageResponse{Message: "Location updated successfully"})
	case errors.Is(err, service.ErrInvalidLocation):
		writeAPIError(w, http.StatusBadRequest, "INVALID_LOCATION", "Location coordinates are missing or invalid")
	case errors.Is(err, service.ErrLocationDriverNotFound):
		writeAPIError(w, http.StatusNotFound, "DRIVER_NOT_FOUND", "No driver exists with this ID")
	default:
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not update driver location")
	}
}

func (h *DriverHandler) GetAvailableDrivers(w http.ResponseWriter, r *http.Request) {
	latitudeText := r.URL.Query().Get("lat")
	longitudeText := r.URL.Query().Get("long")
	if latitudeText == "" || longitudeText == "" {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Latitude and longitude are required")
		return
	}

	latitude, latitudeErr := strconv.ParseFloat(latitudeText, 64)
	longitude, longitudeErr := strconv.ParseFloat(longitudeText, 64)
	if latitudeErr != nil || longitudeErr != nil ||
		math.IsNaN(latitude) || math.IsInf(latitude, 0) ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Latitude and longitude must be valid coordinates")
		return
	}

	drivers, err := h.driverService.GetAvailableDrivers(r.Context(), latitude, longitude)
	if errors.Is(err, service.ErrInvalidDriverInput) {
		writeAPIError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Latitude and longitude must be valid coordinates")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not find available drivers")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(availableDriversResponse{Drivers: drivers})
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{Error: code, Message: message})
}
