package service

import (
	"context"
	"errors"
	"time"

	"myslotmate-backend/internal/lib/realtime"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

type InboxService interface {
	BroadcastMessage(ctx context.Context, hostID uuid.UUID, req BroadcastRequest) (*models.InboxMessage, error)
	GetHostMessages(ctx context.Context, hostID uuid.UUID) ([]*models.InboxMessage, error)
}

type BroadcastRequest struct {
	EventID uuid.UUID
	Message string
}

type inboxService struct {
	inboxRepo     repository.InboxRepository
	eventRepo     repository.EventRepository
	socketService *realtime.SocketService
}

func NewInboxService(ir repository.InboxRepository, er repository.EventRepository, ss *realtime.SocketService) InboxService {
	return &inboxService{
		inboxRepo:     ir,
		eventRepo:     er,
		socketService: ss,
	}
}

func (s *inboxService) BroadcastMessage(ctx context.Context, hostID uuid.UUID, req BroadcastRequest) (*models.InboxMessage, error) {
	// 1. Verify Event Ownership
	evt, err := s.eventRepo.GetByID(ctx, req.EventID)
	if err != nil {
		return nil, err // Error from Repo (e.g. DB error)
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}
	if evt.HostID != hostID {
		return nil, errors.New("unauthorized: you do not own this event")
	}

	// 2. Persist Message
	msg := &models.InboxMessage{
		ID:        uuid.New(),
		EventID:   req.EventID,
		HostID:    hostID,
		Message:   req.Message,
		CreatedAt: time.Now(),
	}

	if err := s.inboxRepo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// 3. Real-time Broadcast via Socket.IO
	// Assume clients (users) join a room named "event_{eventID}" when they view the event
	roomName := "event_" + req.EventID.String()
	s.socketService.BroadcastToRoom(roomName, "inbox_update", map[string]interface{}{
		"message":    req.Message,
		"event_id":   req.EventID,
		"created_at": msg.CreatedAt,
	})

	return msg, nil
}

func (s *inboxService) GetHostMessages(ctx context.Context, hostID uuid.UUID) ([]*models.InboxMessage, error) {
	return s.inboxRepo.ListByHostID(ctx, hostID)
}
