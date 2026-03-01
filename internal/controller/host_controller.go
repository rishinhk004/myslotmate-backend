package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type HostController struct {
	hostService service.HostService
}

func NewHostController(s service.HostService) *HostController {
	return &HostController{hostService: s}
}

func (c *HostController) RegisterRoutes(r chi.Router) {
	r.Route("/hosts", func(r chi.Router) {
		r.Post("/", c.CreateHost)
		r.Get("/me", c.GetMyHostProfile) // Hypothetical endpoint using auth context
	})
}

type CreateHostRequest struct {
	UserID    uuid.UUID `json:"user_id"` // In real usage, get from Auth Context
	Name      string    `json:"name"`
	PhnNumber string    `json:"phn_number"`
}

func (c *HostController) CreateHost(w http.ResponseWriter, r *http.Request) {
	var req CreateHostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	host, err := c.hostService.CreateHost(r.Context(), req.UserID, req.Name, req.PhnNumber)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, host)
}

func (c *HostController) GetMyHostProfile(w http.ResponseWriter, r *http.Request) {
	// Mock: extracting UserID from context/headers
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Missing user_id param (for demo)")
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	host, err := c.hostService.GetHostByUserID(r.Context(), userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if host == nil {
		RespondError(w, http.StatusNotFound, "Host profile not found")
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}
