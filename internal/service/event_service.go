package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type EventService interface {
	CreateEvent(ctx context.Context, hostID uuid.UUID, req EventCreateRequest) (*models.Event, error)
	UpdateEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID, req EventUpdateRequest) (*models.Event, error)
	GetEvent(ctx context.Context, eventID uuid.UUID) (*models.Event, error)
	GetHostEvents(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error)
	GetHostEventsFiltered(ctx context.Context, hostID uuid.UUID, status *models.EventStatus, search string, sortBy string, limit, offset int) ([]*models.Event, error)
	GetCalendarEvents(ctx context.Context, hostID uuid.UUID, start, end time.Time) ([]*models.Event, error)
	GetTodaySchedule(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error)
	PublishEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID) (*models.Event, error)
	PauseEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID) (*models.Event, error)
	ResumeEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID) (*models.Event, error)
	GetEventAttendees(ctx context.Context, eventID uuid.UUID) ([]*models.Booking, error)
	ListPublishedEvents(ctx context.Context, limit, offset int) ([]*models.Event, error)
}

type EventCreateRequest struct {
	Title              string                     `json:"title"`
	HookLine           *string                    `json:"hook_line,omitempty"`
	Mood               *models.EventMood          `json:"mood,omitempty"`
	Description        *string                    `json:"description,omitempty"`
	CoverImageURL      *string                    `json:"cover_image_url,omitempty"`
	GalleryURLs        []string                   `json:"gallery_urls,omitempty"`
	IsOnline           bool                       `json:"is_online"`
	MeetingLink        *string                    `json:"meeting_link,omitempty"` // for online events
	Location           *string                    `json:"location,omitempty"`
	LocationLat        *float64                   `json:"location_lat,omitempty"`
	LocationLng        *float64                   `json:"location_lng,omitempty"`
	GoogleMapsURL      *string                    `json:"google_maps_url,omitempty"` // for location-based events
	DurationMinutes    *int                       `json:"duration_minutes,omitempty"`
	MinGroupSize       *int                       `json:"min_group_size,omitempty"`
	MaxGroupSize       *int                       `json:"max_group_size,omitempty"`
	Capacity           int                        `json:"capacity"`
	PriceCents         *int64                     `json:"price_cents,omitempty"`
	IsFree             bool                       `json:"is_free"`
	Time               time.Time                  `json:"time"`
	EndTime            *time.Time                 `json:"end_time,omitempty"`
	IsRecurring        bool                       `json:"is_recurring"`
	RecurrenceRule     *string                    `json:"recurrence_rule,omitempty"`
	CancellationPolicy *models.CancellationPolicy `json:"cancellation_policy,omitempty"`
	Status             models.EventStatus         `json:"status"` // draft or live
	AISuggestion       *string                    `json:"ai_suggestion,omitempty"`
}

type EventUpdateRequest struct {
	Title              *string                    `json:"title,omitempty"`
	HookLine           *string                    `json:"hook_line,omitempty"`
	Mood               *models.EventMood          `json:"mood,omitempty"`
	Description        *string                    `json:"description,omitempty"`
	CoverImageURL      *string                    `json:"cover_image_url,omitempty"`
	GalleryURLs        []string                   `json:"gallery_urls,omitempty"`
	IsOnline           *bool                      `json:"is_online,omitempty"`
	MeetingLink        *string                    `json:"meeting_link,omitempty"` // for online events
	Location           *string                    `json:"location,omitempty"`
	LocationLat        *float64                   `json:"location_lat,omitempty"`
	LocationLng        *float64                   `json:"location_lng,omitempty"`
	GoogleMapsURL      *string                    `json:"google_maps_url,omitempty"` // for location-based events
	DurationMinutes    *int                       `json:"duration_minutes,omitempty"`
	MinGroupSize       *int                       `json:"min_group_size,omitempty"`
	MaxGroupSize       *int                       `json:"max_group_size,omitempty"`
	Capacity           *int                       `json:"capacity,omitempty"`
	PriceCents         *int64                     `json:"price_cents,omitempty"`
	IsFree             *bool                      `json:"is_free,omitempty"`
	Time               *time.Time                 `json:"time,omitempty"`
	EndTime            *time.Time                 `json:"end_time,omitempty"`
	IsRecurring        *bool                      `json:"is_recurring,omitempty"`
	RecurrenceRule     *string                    `json:"recurrence_rule,omitempty"`
	CancellationPolicy *models.CancellationPolicy `json:"cancellation_policy,omitempty"`
}

type eventService struct {
	eventRepo   repository.EventRepository
	bookingRepo repository.BookingRepository
	dispatcher  *event.Dispatcher
}

var ErrInvalidEventMood = errors.New("invalid event mood")

func NewEventService(er repository.EventRepository, br repository.BookingRepository, d *event.Dispatcher) EventService {
	return &eventService{
		eventRepo:   er,
		bookingRepo: br,
		dispatcher:  d,
	}
}

func normalizeEventMood(mood *models.EventMood) (*models.EventMood, error) {
	canonical, err := models.NormalizeEventMood(mood)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEventMood, err)
	}
	return canonical, nil
}

func (s *eventService) CreateEvent(ctx context.Context, hostID uuid.UUID, req EventCreateRequest) (*models.Event, error) {
	normalizedMood, err := normalizeEventMood(req.Mood)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	status := req.Status
	if status == "" {
		status = models.EventStatusDraft
	}

	newEvent := &models.Event{
		ID:                 uuid.New(),
		HostID:             hostID,
		Title:              req.Title,
		HookLine:           req.HookLine,
		Mood:               normalizedMood,
		Description:        req.Description,
		CoverImageURL:      req.CoverImageURL,
		GalleryURLs:        pq.StringArray(req.GalleryURLs),
		IsOnline:           req.IsOnline,
		MeetingLink:        req.MeetingLink,
		Location:           req.Location,
		LocationLat:        req.LocationLat,
		LocationLng:        req.LocationLng,
		GoogleMapsURL:      req.GoogleMapsURL,
		DurationMinutes:    req.DurationMinutes,
		MinGroupSize:       req.MinGroupSize,
		MaxGroupSize:       req.MaxGroupSize,
		Capacity:           req.Capacity,
		PriceCents:         req.PriceCents,
		IsFree:             req.IsFree,
		Time:               req.Time,
		EndTime:            req.EndTime,
		IsRecurring:        req.IsRecurring,
		RecurrenceRule:     req.RecurrenceRule,
		CancellationPolicy: req.CancellationPolicy,
		Status:             status,
		AISuggestion:       req.AISuggestion,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if status == models.EventStatusLive {
		newEvent.PublishedAt = &now
	}

	if err := s.eventRepo.Create(ctx, newEvent); err != nil {
		return nil, err
	}

	s.dispatcher.Publish(event.EventCreated, newEvent)

	return newEvent, nil
}

func (s *eventService) UpdateEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID, req EventUpdateRequest) (*models.Event, error) {
	evt, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}
	if evt.HostID != hostID {
		return nil, errors.New("unauthorized: you do not own this event")
	}

	if req.Title != nil {
		evt.Title = *req.Title
	}
	if req.HookLine != nil {
		evt.HookLine = req.HookLine
	}
	if req.Mood != nil {
		normalizedMood, err := normalizeEventMood(req.Mood)
		if err != nil {
			return nil, err
		}
		evt.Mood = normalizedMood
	}
	if req.Description != nil {
		evt.Description = req.Description
	}
	if req.CoverImageURL != nil {
		evt.CoverImageURL = req.CoverImageURL
	}
	if req.GalleryURLs != nil {
		evt.GalleryURLs = pq.StringArray(req.GalleryURLs)
	}
	if req.IsOnline != nil {
		evt.IsOnline = *req.IsOnline
	}
	if req.MeetingLink != nil {
		evt.MeetingLink = req.MeetingLink
	}
	if req.Location != nil {
		evt.Location = req.Location
	}
	if req.LocationLat != nil {
		evt.LocationLat = req.LocationLat
	}
	if req.LocationLng != nil {
		evt.LocationLng = req.LocationLng
	}
	if req.GoogleMapsURL != nil {
		evt.GoogleMapsURL = req.GoogleMapsURL
	}
	if req.DurationMinutes != nil {
		evt.DurationMinutes = req.DurationMinutes
	}
	if req.MinGroupSize != nil {
		evt.MinGroupSize = req.MinGroupSize
	}
	if req.MaxGroupSize != nil {
		evt.MaxGroupSize = req.MaxGroupSize
	}
	if req.Capacity != nil {
		evt.Capacity = *req.Capacity
	}
	if req.PriceCents != nil {
		evt.PriceCents = req.PriceCents
	}
	if req.IsFree != nil {
		evt.IsFree = *req.IsFree
	}
	if req.Time != nil {
		evt.Time = *req.Time
	}
	if req.EndTime != nil {
		evt.EndTime = req.EndTime
	}
	if req.IsRecurring != nil {
		evt.IsRecurring = *req.IsRecurring
	}
	if req.RecurrenceRule != nil {
		evt.RecurrenceRule = req.RecurrenceRule
	}
	if req.CancellationPolicy != nil {
		evt.CancellationPolicy = req.CancellationPolicy
	}

	if err := s.eventRepo.Update(ctx, evt); err != nil {
		return nil, err
	}
	return evt, nil
}

func (s *eventService) GetEvent(ctx context.Context, eventID uuid.UUID) (*models.Event, error) {
	return s.eventRepo.GetByID(ctx, eventID)
}

func (s *eventService) GetHostEvents(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error) {
	return s.eventRepo.ListByHostID(ctx, hostID)
}

func (s *eventService) GetHostEventsFiltered(ctx context.Context, hostID uuid.UUID, status *models.EventStatus, search string, sortBy string, limit, offset int) ([]*models.Event, error) {
	return s.eventRepo.ListByHostIDFiltered(ctx, hostID, status, search, sortBy, limit, offset)
}

func (s *eventService) GetCalendarEvents(ctx context.Context, hostID uuid.UUID, start, end time.Time) ([]*models.Event, error) {
	return s.eventRepo.ListByDateRange(ctx, hostID, start, end)
}

func (s *eventService) GetTodaySchedule(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error) {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	return s.eventRepo.ListTodayByHostID(ctx, hostID, dayStart, dayEnd)
}

func (s *eventService) PublishEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID) (*models.Event, error) {
	evt, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}
	if evt.HostID != hostID {
		return nil, errors.New("unauthorized")
	}
	if err := s.eventRepo.UpdateStatus(ctx, eventID, models.EventStatusLive); err != nil {
		return nil, err
	}
	evt.Status = models.EventStatusLive
	now := time.Now()
	evt.PublishedAt = &now
	return evt, nil
}

func (s *eventService) PauseEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID) (*models.Event, error) {
	evt, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}
	if evt.HostID != hostID {
		return nil, errors.New("unauthorized")
	}
	if err := s.eventRepo.UpdateStatus(ctx, eventID, models.EventStatusPaused); err != nil {
		return nil, err
	}
	evt.Status = models.EventStatusPaused
	return evt, nil
}

func (s *eventService) ResumeEvent(ctx context.Context, eventID uuid.UUID, hostID uuid.UUID) (*models.Event, error) {
	evt, err := s.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}
	if evt.HostID != hostID {
		return nil, errors.New("unauthorized")
	}
	if err := s.eventRepo.UpdateStatus(ctx, eventID, models.EventStatusLive); err != nil {
		return nil, err
	}
	evt.Status = models.EventStatusLive
	return evt, nil
}

func (s *eventService) GetEventAttendees(ctx context.Context, eventID uuid.UUID) ([]*models.Booking, error) {
	return s.bookingRepo.ListByEventID(ctx, eventID)
}

func (s *eventService) ListPublishedEvents(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	return s.eventRepo.ListPublished(ctx, limit, offset)
}
