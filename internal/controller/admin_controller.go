package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/auth"
	"myslotmate-backend/internal/service"

	fbauth "firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// AdminController handles admin-only host-application management.
type AdminController struct {
	hostService  service.HostService
	firebaseAuth *fbauth.Client
	adminEmail   string
}

func NewAdminController(hs service.HostService, fa *fbauth.Client, adminEmail string) *AdminController {
	return &AdminController{
		hostService:  hs,
		firebaseAuth: fa,
		adminEmail:   adminEmail,
	}
}

func (c *AdminController) RegisterRoutes(r chi.Router) {
	r.Route("/admin/hosts", func(r chi.Router) {
		// All routes in this group require admin authentication
		r.Use(auth.IsAdmin(c.firebaseAuth, c.adminEmail))

		r.Get("/applications", c.ListPendingApplications)
		r.Post("/{hostID}/approve", c.ApproveApplication)
		r.Post("/{hostID}/reject", c.RejectApplication)
	})
}

// ── Request types ───────────────────────────────────────────────────────────

type RejectApplicationRequestBody struct {
	Reason string `json:"reason"`
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (c *AdminController) ListPendingApplications(w http.ResponseWriter, r *http.Request) {
	hosts, err := c.hostService.ListPendingApplications(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondSuccess(w, http.StatusOK, hosts)
}

func (c *AdminController) ApproveApplication(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "hostID")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	host, err := c.hostService.ApproveApplication(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}

func (c *AdminController) RejectApplication(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "hostID")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	var req RejectApplicationRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	host, err := c.hostService.RejectApplication(r.Context(), hostID, req.Reason)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, host)
}
