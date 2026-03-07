package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type SupportService interface {
	CreateTicket(ctx context.Context, userID uuid.UUID, req CreateSupportTicketRequest) (*models.SupportTicket, error)
	GetTicket(ctx context.Context, ticketID uuid.UUID) (*models.SupportTicket, error)
	GetUserTickets(ctx context.Context, userID uuid.UUID) ([]*models.SupportTicket, error)
	AddMessage(ctx context.Context, ticketID uuid.UUID, message string) (*models.SupportTicket, error)
	ResolveTicket(ctx context.Context, ticketID uuid.UUID) error
}

type CreateSupportTicketRequest struct {
	Category       models.SupportCategory `json:"category"`
	Subject        string                 `json:"subject"`
	Message        string                 `json:"message"`
	ReportedUserID *uuid.UUID             `json:"reported_user_id,omitempty"`

	// Report-specific fields
	EventID      *uuid.UUID           `json:"event_id,omitempty"`
	SessionDate  *time.Time           `json:"session_date,omitempty"`
	ReportReason *models.ReportReason `json:"report_reason,omitempty"`
	EvidenceURLs []string             `json:"evidence_urls,omitempty"`
	IsUrgent     bool                 `json:"is_urgent"`
}

type supportService struct {
	supportRepo repository.SupportRepository
}

func NewSupportService(sr repository.SupportRepository) SupportService {
	return &supportService{supportRepo: sr}
}

func (s *supportService) CreateTicket(ctx context.Context, userID uuid.UUID, req CreateSupportTicketRequest) (*models.SupportTicket, error) {
	if req.Subject == "" {
		return nil, errors.New("subject is required")
	}
	if req.Message == "" {
		return nil, errors.New("message is required")
	}

	initialMessages := []models.SupportTicketMessage{
		{Sender: "user", Text: req.Message, CreatedAt: time.Now()},
	}
	messagesJSON, err := json.Marshal(initialMessages)
	if err != nil {
		return nil, err
	}

	ticket := &models.SupportTicket{
		ID:             uuid.New(),
		UserID:         userID,
		Category:       req.Category,
		Subject:        req.Subject,
		Messages:       messagesJSON,
		Status:         "open",
		ReportedUserID: req.ReportedUserID,
		EventID:        req.EventID,
		SessionDate:    req.SessionDate,
		ReportReason:   req.ReportReason,
		EvidenceURLs:   pq.StringArray(req.EvidenceURLs),
		IsUrgent:       req.IsUrgent,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.supportRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *supportService) GetTicket(ctx context.Context, ticketID uuid.UUID) (*models.SupportTicket, error) {
	ticket, err := s.supportRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, errors.New("ticket not found")
	}
	return ticket, nil
}

func (s *supportService) GetUserTickets(ctx context.Context, userID uuid.UUID) ([]*models.SupportTicket, error) {
	return s.supportRepo.ListByUserID(ctx, userID)
}

func (s *supportService) AddMessage(ctx context.Context, ticketID uuid.UUID, message string) (*models.SupportTicket, error) {
	ticket, err := s.supportRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket == nil {
		return nil, errors.New("ticket not found")
	}

	var messages []models.SupportTicketMessage
	if err := json.Unmarshal(ticket.Messages, &messages); err != nil {
		return nil, err
	}
	messages = append(messages, models.SupportTicketMessage{
		Sender:    "user",
		Text:      message,
		CreatedAt: time.Now(),
	})
	updatedJSON, err := json.Marshal(messages)
	if err != nil {
		return nil, err
	}

	ticket.Messages = updatedJSON
	ticket.UpdatedAt = time.Now()

	if err := s.supportRepo.UpdateMessages(ctx, ticketID, ticket.Messages); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *supportService) ResolveTicket(ctx context.Context, ticketID uuid.UUID) error {
	return s.supportRepo.UpdateStatus(ctx, ticketID, "resolved")
}
