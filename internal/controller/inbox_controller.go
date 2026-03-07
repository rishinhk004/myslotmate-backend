package controller

import (
	"encoding/json"
	"net/http"

	"myslotmate-backend/internal/models"
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
		r.Post("/send", c.SendMessage)
		r.Post("/broadcast", c.BroadcastMessage)
		r.Get("/event/{eventID}", c.GetEventMessages)
		r.Get("/host/{hostID}", c.GetHostMessages)
		r.Post("/{messageID}/read", c.MarkRead)
	})
}

type SendMessageRequestBody struct {
	EventID       uuid.UUID                `json:"event_id"`
	SenderType    models.MessageSenderType `json:"sender_type"`
	SenderID      *uuid.UUID               `json:"sender_id,omitempty"`
	Message       string                   `json:"message"`
	AttachmentURL *string                  `json:"attachment_url,omitempty"`
}

type BroadcastRequestBody struct {
	HostID  uuid.UUID `json:"host_id"`
	EventID uuid.UUID `json:"event_id"`
	Message string    `json:"message"`
}

func (c *InboxController) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req SendMessageRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.SendMessageRequest{
		EventID:       req.EventID,
		SenderType:    req.SenderType,
		SenderID:      req.SenderID,
		Message:       req.Message,
		AttachmentURL: req.AttachmentURL,
	}

	msg, err := c.inboxService.SendMessage(r.Context(), svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, msg)
}

func (c *InboxController) BroadcastMessage(w http.ResponseWriter, r *http.Request) {
	var req BroadcastRequestBody
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

func (c *InboxController) GetEventMessages(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	msgs, err := c.inboxService.GetEventMessages(r.Context(), eventID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, msgs)
}

func (c *InboxController) GetHostMessages(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
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

func (c *InboxController) MarkRead(w http.ResponseWriter, r *http.Request) {
	messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	if err := c.inboxService.MarkRead(r.Context(), messageID); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, map[string]string{"message": "marked as read"})
}
