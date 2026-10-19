
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"safe-ride/internal/service"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

type createUserRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

func (h *UserHandler) CreateUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req createUserRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	user, err := h.userService.CreateUser(
		r.Context(),
		req.Name,
		req.Phone,
		req.Password,
	)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		http.Error(w, "could not create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(user); err != nil {
		return
	}
}
