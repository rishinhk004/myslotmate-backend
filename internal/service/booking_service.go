package service

import (
	"context"
	"errors"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

type BookingService interface {
	CreateBooking(ctx context.Context, userID uuid.UUID, req BookingCreateRequest) (*models.Booking, error)
	GetUserBookings(ctx context.Context, userID uuid.UUID) ([]*models.Booking, error)
}

type BookingCreateRequest struct {
	EventID  uuid.UUID
	Quantity int
}

type bookingService struct {
	bookingRepo repository.BookingRepository
	eventRepo   repository.EventRepository
	dispatcher  *event.Dispatcher
}

func NewBookingService(br repository.BookingRepository, er repository.EventRepository, d *event.Dispatcher) BookingService {
	return &bookingService{
		bookingRepo: br,
		eventRepo:   er,
		dispatcher:  d,
	}
}

func (s *bookingService) CreateBooking(ctx context.Context, userID uuid.UUID, req BookingCreateRequest) (*models.Booking, error) {
	evt, err := s.eventRepo.GetByID(ctx, req.EventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}

	// Overbooking Prevention
	currentBooked, err := s.bookingRepo.GetTotalBookedQuantity(ctx, req.EventID)
	if err != nil {
		return nil, err
	}

	if currentBooked+req.Quantity > evt.Capacity {
		return nil, errors.New("event capacity exceeded")
	}

	// Strategy Pattern Logic could go here
	// Platform Fee 15%
	// Hardcoded for now, but should come from PlatformSettings
	// Assuming price mechanism exists (not in basic schema, but implied by amount_cents)
	// For now, setting dummy values or 0 if free
	var amountCents int64 = 1000 // Dummy: 10.00 currency units per ticket
	totalAmount := amountCents * int64(req.Quantity)
	serviceFee := int64(float64(totalAmount) * 0.15)
	netEarning := totalAmount - serviceFee

	// Default to pending until payment
	newBooking := &models.Booking{
		ID:              uuid.New(),
		EventID:         req.EventID,
		UserID:          userID,
		Quantity:        req.Quantity,
		Status:          models.BookingStatusPending, // Default to pending until payment
		AmountCents:     &totalAmount,
		ServiceFeeCents: &serviceFee,
		NetEarningCents: &netEarning,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.bookingRepo.Create(ctx, newBooking); err != nil {
		return nil, err
	}

	s.dispatcher.Publish(event.BookingCreated, newBooking)

	return newBooking, nil
}

func (s *bookingService) GetUserBookings(ctx context.Context, userID uuid.UUID) ([]*models.Booking, error) {
	return s.bookingRepo.ListByUserID(ctx, userID)
}
