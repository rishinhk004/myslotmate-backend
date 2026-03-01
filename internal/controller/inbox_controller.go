package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type InboxController struct {
	inboxService service.InboxService
}

func NewInboxController(s service.InboxService) *InboxController {
	return &InboxController{inboxService: s}
}

func (c *InboxController) RegisterRoutes(r chi.Router) {
	r.Route("/inbox", func(r chi.Router) {
		r.Post("/broadcast", c.BroadcastMessage)
		r.Get("/host/{hostID}", c.GetHostMessages)
	})
}

type BroadcastRequest struct {
	HostID  uuid.UUID `json:"host_id"` // Auth context
	EventID uuid.UUID `json:"event_id"`
	Message string    `json:"message"`
}

func (c *InboxController) BroadcastMessage(w http.ResponseWriter, r *http.Request) {
	var req BroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.BroadcastRequest{
		EventID: req.EventID,
		Message: req.Message,
	}

	msg, err := c.inboxService.BroadcastMessage(r.Context(), req.HostID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, msg)
}

func (c *InboxController) GetHostMessages(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "hostID")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	msgs, err := c.inboxService.GetHostMessages(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, msgs)
}
