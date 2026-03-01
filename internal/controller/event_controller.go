package controller

import (
	"encoding/json"
	"net/http"
	"time"

	"myslotmate-backend/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type EventController struct {
	eventService service.EventService
}

func NewEventController(s service.EventService) *EventController {
	return &EventController{eventService: s}
}

func (c *EventController) RegisterRoutes(r chi.Router) {
	r.Route("/events", func(r chi.Router) {
		r.Post("/", c.CreateEvent)
		r.Get("/host/{hostID}", c.GetHostEvents)
	})
}

type CreateEventRequest struct {
	HostID       uuid.UUID  `json:"host_id"` // In real usage, get from Auth Context associated Host
	Name         string     `json:"name"`
	Time         time.Time  `json:"time"`
	EndTime      *time.Time `json:"end_time"`
	Capacity     int        `json:"capacity"`
	AISuggestion *string    `json:"ai_suggestion"`
}

func (c *EventController) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.EventCreateRequest{
		Name:         req.Name,
		Time:         req.Time,
		EndTime:      req.EndTime,
		Capacity:     req.Capacity,
		AISuggestion: req.AISuggestion,
	}

	event, err := c.eventService.CreateEvent(r.Context(), req.HostID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, event)
}

func (c *EventController) GetHostEvents(w http.ResponseWriter, r *http.Request) {
	hostIDStr := chi.URLParam(r, "hostID")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	events, err := c.eventService.GetHostEvents(r.Context(), hostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, events)
}
