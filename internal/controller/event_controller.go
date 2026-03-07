package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"myslotmate-backend/internal/models"
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
		r.Put("/{eventID}", c.UpdateEvent)
		r.Get("/{eventID}", c.GetEvent)
		r.Get("/host/{hostID}", c.GetHostEvents)
		r.Get("/host/{hostID}/filtered", c.GetHostEventsFiltered)
		r.Get("/calendar/{hostID}", c.GetCalendarEvents)
		r.Post("/{eventID}/publish", c.PublishEvent)
		r.Post("/{eventID}/pause", c.PauseEvent)
		r.Post("/{eventID}/resume", c.ResumeEvent)
		r.Get("/{eventID}/attendees", c.GetEventAttendees)
	})
}

// ── Request types ───────────────────────────────────────────────────────────

type EventCreateRequestBody struct {
	HostID             uuid.UUID                  `json:"host_id"`
	Title              string                     `json:"title"`
	HookLine           *string                    `json:"hook_line,omitempty"`
	Mood               *models.EventMood          `json:"mood,omitempty"`
	Description        *string                    `json:"description,omitempty"`
	CoverImageURL      *string                    `json:"cover_image_url,omitempty"`
	GalleryURLs        []string                   `json:"gallery_urls,omitempty"`
	Time               time.Time                  `json:"time"`
	EndTime            *time.Time                 `json:"end_time,omitempty"`
	IsOnline           bool                       `json:"is_online"`
	Location           *string                    `json:"location,omitempty"`
	LocationLat        *float64                   `json:"location_lat,omitempty"`
	LocationLng        *float64                   `json:"location_lng,omitempty"`
	DurationMinutes    *int                       `json:"duration_minutes,omitempty"`
	Capacity           int                        `json:"capacity"`
	MinGroupSize       *int                       `json:"min_group_size,omitempty"`
	MaxGroupSize       *int                       `json:"max_group_size,omitempty"`
	PriceCents         *int64                     `json:"price_cents,omitempty"`
	IsFree             bool                       `json:"is_free"`
	IsRecurring        bool                       `json:"is_recurring"`
	RecurrenceRule     *string                    `json:"recurrence_rule,omitempty"`
	CancellationPolicy *models.CancellationPolicy `json:"cancellation_policy,omitempty"`
	AISuggestion       *string                    `json:"ai_suggestion,omitempty"`
}

type EventUpdateRequestBody struct {
	Title              *string                    `json:"title,omitempty"`
	HookLine           *string                    `json:"hook_line,omitempty"`
	Mood               *models.EventMood          `json:"mood,omitempty"`
	Description        *string                    `json:"description,omitempty"`
	CoverImageURL      *string                    `json:"cover_image_url,omitempty"`
	GalleryURLs        []string                   `json:"gallery_urls,omitempty"`
	Time               *time.Time                 `json:"time,omitempty"`
	EndTime            *time.Time                 `json:"end_time,omitempty"`
	IsOnline           *bool                      `json:"is_online,omitempty"`
	Location           *string                    `json:"location,omitempty"`
	LocationLat        *float64                   `json:"location_lat,omitempty"`
	LocationLng        *float64                   `json:"location_lng,omitempty"`
	DurationMinutes    *int                       `json:"duration_minutes,omitempty"`
	Capacity           *int                       `json:"capacity,omitempty"`
	MinGroupSize       *int                       `json:"min_group_size,omitempty"`
	MaxGroupSize       *int                       `json:"max_group_size,omitempty"`
	PriceCents         *int64                     `json:"price_cents,omitempty"`
	IsFree             *bool                      `json:"is_free,omitempty"`
	IsRecurring        *bool                      `json:"is_recurring,omitempty"`
	RecurrenceRule     *string                    `json:"recurrence_rule,omitempty"`
	CancellationPolicy *models.CancellationPolicy `json:"cancellation_policy,omitempty"`
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (c *EventController) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req EventCreateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.EventCreateRequest{
		Title:              req.Title,
		HookLine:           req.HookLine,
		Mood:               req.Mood,
		Description:        req.Description,
		CoverImageURL:      req.CoverImageURL,
		GalleryURLs:        req.GalleryURLs,
		Time:               req.Time,
		EndTime:            req.EndTime,
		IsOnline:           req.IsOnline,
		Location:           req.Location,
		LocationLat:        req.LocationLat,
		LocationLng:        req.LocationLng,
		DurationMinutes:    req.DurationMinutes,
		Capacity:           req.Capacity,
		MinGroupSize:       req.MinGroupSize,
		MaxGroupSize:       req.MaxGroupSize,
		PriceCents:         req.PriceCents,
		IsFree:             req.IsFree,
		IsRecurring:        req.IsRecurring,
		RecurrenceRule:     req.RecurrenceRule,
		CancellationPolicy: req.CancellationPolicy,
		AISuggestion:       req.AISuggestion,
	}

	evt, err := c.eventService.CreateEvent(r.Context(), req.HostID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusCreated, evt)
}

func (c *EventController) UpdateEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var body struct {
		HostID uuid.UUID `json:"host_id"`
		EventUpdateRequestBody
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	svcReq := service.EventUpdateRequest{
		Title:              body.Title,
		HookLine:           body.HookLine,
		Mood:               body.Mood,
		Description:        body.Description,
		CoverImageURL:      body.CoverImageURL,
		GalleryURLs:        body.GalleryURLs,
		Time:               body.Time,
		EndTime:            body.EndTime,
		IsOnline:           body.IsOnline,
		Location:           body.Location,
		LocationLat:        body.LocationLat,
		LocationLng:        body.LocationLng,
		DurationMinutes:    body.DurationMinutes,
		Capacity:           body.Capacity,
		MinGroupSize:       body.MinGroupSize,
		MaxGroupSize:       body.MaxGroupSize,
		PriceCents:         body.PriceCents,
		IsFree:             body.IsFree,
		IsRecurring:        body.IsRecurring,
		RecurrenceRule:     body.RecurrenceRule,
		CancellationPolicy: body.CancellationPolicy,
	}

	evt, err := c.eventService.UpdateEvent(r.Context(), eventID, body.HostID, svcReq)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, evt)
}

func (c *EventController) GetEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	evt, err := c.eventService.GetEvent(r.Context(), eventID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, evt)
}

func (c *EventController) GetHostEvents(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
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

func (c *EventController) GetHostEventsFiltered(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	search := r.URL.Query().Get("search")
	statusStr := r.URL.Query().Get("status")
	var status *models.EventStatus
	if statusStr != "" {
		s := models.EventStatus(statusStr)
		status = &s
	}
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "created_at"
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}

	events, err := c.eventService.GetHostEventsFiltered(r.Context(), hostID, status, search, sortBy, limit, offset)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, events)
}

func (c *EventController) GetCalendarEvents(w http.ResponseWriter, r *http.Request) {
	hostID, err := uuid.Parse(chi.URLParam(r, "hostID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid host ID")
		return
	}

	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	if startStr == "" || endStr == "" {
		RespondError(w, http.StatusBadRequest, "start and end query params required (RFC3339)")
		return
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid start time format")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid end time format")
		return
	}

	events, err := c.eventService.GetCalendarEvents(r.Context(), hostID, start, end)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, events)
}

func (c *EventController) PublishEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var body struct {
		HostID uuid.UUID `json:"host_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	evt, err := c.eventService.PublishEvent(r.Context(), eventID, body.HostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, evt)
}

func (c *EventController) PauseEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var body struct {
		HostID uuid.UUID `json:"host_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	evt, err := c.eventService.PauseEvent(r.Context(), eventID, body.HostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, evt)
}

func (c *EventController) ResumeEvent(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var body struct {
		HostID uuid.UUID `json:"host_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	evt, err := c.eventService.ResumeEvent(r.Context(), eventID, body.HostID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, evt)
}

func (c *EventController) GetEventAttendees(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	attendees, err := c.eventService.GetEventAttendees(r.Context(), eventID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondSuccess(w, http.StatusOK, attendees)
}
