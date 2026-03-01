package service

import (
	"context"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

type EventService interface {
	CreateEvent(ctx context.Context, hostID uuid.UUID, req EventCreateRequest) (*models.Event, error)
	GetHostEvents(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error)
}

type EventCreateRequest struct {
	Name         string
	Time         time.Time
	EndTime      *time.Time
	Capacity     int
	AISuggestion *string
}

type eventService struct {
	eventRepo  repository.EventRepository
	dispatcher *event.Dispatcher
}

func NewEventService(er repository.EventRepository, d *event.Dispatcher) EventService {
	return &eventService{
		eventRepo:  er,
		dispatcher: d,
	}
}

func (s *eventService) CreateEvent(ctx context.Context, hostID uuid.UUID, req EventCreateRequest) (*models.Event, error) {
	newEvent := &models.Event{
		ID:           uuid.New(),
		HostID:       hostID,
		Name:         req.Name,
		Time:         req.Time,
		EndTime:      req.EndTime,
		Capacity:     req.Capacity,
		AISuggestion: req.AISuggestion,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.eventRepo.Create(ctx, newEvent); err != nil {
		return nil, err
	}

	s.dispatcher.Publish(event.EventCreated, newEvent)

	return newEvent, nil
}

func (s *eventService) GetHostEvents(ctx context.Context, hostID uuid.UUID) ([]*models.Event, error) {
	return s.eventRepo.ListByHostID(ctx, hostID)
}
