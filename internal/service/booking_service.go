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
)

type BookingService interface {
	CreateBooking(ctx context.Context, userID uuid.UUID, req BookingCreateRequest) (*models.Booking, error)
	ConfirmBooking(ctx context.Context, bookingID uuid.UUID) (*models.Booking, error)
	CancelBooking(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) (*models.Booking, error)
	GetUserBookings(ctx context.Context, userID uuid.UUID) ([]*models.Booking, error)
}

type BookingCreateRequest struct {
	EventID        uuid.UUID
	Quantity       int
	IdempotencyKey string
}

type bookingService struct {
	bookingRepo repository.BookingRepository
	eventRepo   repository.EventRepository
	accountRepo repository.AccountRepository
	paymentRepo repository.PaymentRepository
	payoutRepo  repository.PayoutRepository
	hostRepo    repository.HostRepository
	dispatcher  *event.Dispatcher
}

func NewBookingService(
	br repository.BookingRepository,
	er repository.EventRepository,
	ar repository.AccountRepository,
	pmr repository.PaymentRepository,
	pr repository.PayoutRepository,
	hr repository.HostRepository,
	d *event.Dispatcher,
) BookingService {
	return &bookingService{
		bookingRepo: br,
		eventRepo:   er,
		accountRepo: ar,
		paymentRepo: pmr,
		payoutRepo:  pr,
		hostRepo:    hr,
		dispatcher:  d,
	}
}

func (s *bookingService) CreateBooking(ctx context.Context, userID uuid.UUID, req BookingCreateRequest) (*models.Booking, error) {
	// 1. Idempotency check
	if req.IdempotencyKey != "" {
		existing, err := s.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ReferenceID != nil {
			// Return the existing booking
			return s.bookingRepo.GetByID(ctx, *existing.ReferenceID)
		}
	}

	// 2. Fraud check
	flagged, err := s.payoutRepo.HasActiveFraudFlag(ctx, userID)
	if err != nil {
		return nil, err
	}
	if flagged {
		return nil, errors.New("your account is blocked due to suspicious activity")
	}

	// 3. Validate event
	evt, err := s.eventRepo.GetByID(ctx, req.EventID)
	if err != nil {
		return nil, err
	}
	if evt == nil {
		return nil, errors.New("event not found")
	}

	// 4. Overbooking prevention
	currentBooked, err := s.bookingRepo.GetTotalBookedQuantity(ctx, req.EventID)
	if err != nil {
		return nil, err
	}
	if currentBooked+req.Quantity > evt.Capacity {
		return nil, errors.New("event capacity exceeded")
	}

	// 5. Get platform fee config dynamically from PlatformSettings
	feeConfig, err := s.payoutRepo.GetPlatformFeeConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Price per ticket — placeholder until Event has a price field
	var pricePerTicketCents int64 = 1000 // 10.00 currency units
	totalAmount := pricePerTicketCents * int64(req.Quantity)
	serviceFee := totalAmount * int64(feeConfig.PlatformPercentage) / 100
	netEarning := totalAmount - serviceFee

	// 6. Get user account
	userAccount, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerUser, userID)
	if err != nil {
		return nil, err
	}
	if userAccount == nil {
		return nil, errors.New("user account not found")
	}

	// 7. Debit user wallet
	if err := s.accountRepo.Debit(ctx, userAccount.ID, totalAmount); err != nil {
		if errors.Is(err, repository.ErrInsufficientBalance) {
			return nil, errors.New("insufficient wallet balance; please top up first")
		}
		return nil, err
	}

	// 8. Create booking
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey != "" {
		// store on booking too
	}
	newBooking := &models.Booking{
		ID:              uuid.New(),
		EventID:         req.EventID,
		UserID:          userID,
		Quantity:        req.Quantity,
		Status:          models.BookingStatusPending,
		IdempotencyKey:  &idempotencyKey,
		AmountCents:     &totalAmount,
		ServiceFeeCents: &serviceFee,
		NetEarningCents: &netEarning,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.bookingRepo.Create(ctx, newBooking); err != nil {
		// Credit back on failure
		_ = s.accountRepo.Credit(ctx, userAccount.ID, totalAmount)
		return nil, err
	}

	// 9. Create payment record for the booking
	paymentIdempotencyKey := idempotencyKey
	if paymentIdempotencyKey == "" {
		paymentIdempotencyKey = fmt.Sprintf("booking_%s", newBooking.ID)
	}
	displayRef := fmt.Sprintf("BK-%05d", time.Now().UnixMilli()%100000)
	bookingPayment := &models.Payment{
		ID:               uuid.New(),
		IdempotencyKey:   paymentIdempotencyKey,
		AccountID:        userAccount.ID,
		Type:             models.PaymentTypeBooking,
		ReferenceID:      &newBooking.ID,
		AmountCents:      totalAmount,
		Status:           models.PaymentStatusCompleted,
		RetryCount:       0,
		DisplayReference: &displayRef,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.paymentRepo.Create(ctx, bookingPayment); err != nil {
		// Non-critical — booking still valid, payment record can be reconciled
		// Log this in production
	}
	newBooking.PaymentID = &bookingPayment.ID

	// 10. Credit host wallet with net earning
	host, err := s.hostRepo.GetByID(ctx, evt.HostID)
	if err == nil && host != nil {
		hostAccount, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, host.ID)
		if err == nil && hostAccount != nil {
			_ = s.accountRepo.Credit(ctx, hostAccount.ID, netEarning)

			// Update host earnings aggregate
			_ = s.payoutRepo.IncrementEarnings(ctx, host.ID, netEarning)
			_ = s.payoutRepo.AddPendingClearance(ctx, host.ID, netEarning)
		}
	}

	// 11. Credit platform wallet with service fee
	platformAccount, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerPlatform, uuid.Nil)
	if err == nil && platformAccount != nil {
		_ = s.accountRepo.Credit(ctx, platformAccount.ID, serviceFee)
	}

	// 12. Publish event
	s.dispatcher.Publish(event.BookingCreated, newBooking)

	return newBooking, nil
}

func (s *bookingService) ConfirmBooking(ctx context.Context, bookingID uuid.UUID) (*models.Booking, error) {
	fmt.Printf("[BOOKING] ConfirmBooking: bookingID=%s\n", bookingID)

	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		fmt.Printf("[BOOKING] ConfirmBooking: GetByID error: %v\n", err)
		return nil, err
	}
	if booking == nil {
		fmt.Printf("[BOOKING] ConfirmBooking: booking not found\n")
		return nil, errors.New("booking not found")
	}
	if booking.Status != models.BookingStatusPending {
		fmt.Printf("[BOOKING] ConfirmBooking: cannot confirm from status=%s\n", booking.Status)
		return nil, fmt.Errorf("booking cannot be confirmed from status: %s", booking.Status)
	}

	fmt.Printf("[BOOKING] ConfirmBooking: status=%s -> confirmed, quantity=%d\n", booking.Status, booking.Quantity)
	booking.Status = models.BookingStatusConfirmed
	booking.UpdatedAt = time.Now()

	if err := s.bookingRepo.UpdateStatus(ctx, bookingID, models.BookingStatusConfirmed); err != nil {
		fmt.Printf("[BOOKING] ConfirmBooking: UpdateStatus error: %v\n", err)
		return nil, err
	}

	// Increment event's total_bookings counter
	fmt.Printf("[BOOKING] ConfirmBooking: incrementing event %s by %d\n", booking.EventID, booking.Quantity)
	if err := s.eventRepo.IncrementBookingCount(ctx, booking.EventID, booking.Quantity); err != nil {
		fmt.Printf("[BOOKING] ERROR: IncrementBookingCount failed: %v\n", err)
		// Non-critical — booking already confirmed, counter reconciliation can happen in background
	}

	fmt.Printf("[BOOKING] ConfirmBooking: SUCCESS\n")
	s.dispatcher.Publish(event.BookingConfirmed, booking)
	return booking, nil
}

func (s *bookingService) CancelBooking(ctx context.Context, bookingID uuid.UUID, userID uuid.UUID) (*models.Booking, error) {
	booking, err := s.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, errors.New("booking not found")
	}
	if booking.UserID != userID {
		return nil, errors.New("not authorized to cancel this booking")
	}
	if booking.Status == models.BookingStatusCancelled || booking.Status == models.BookingStatusRefunded {
		return nil, errors.New("booking is already cancelled or refunded")
	}

	now := time.Now()

	// 1. Update booking status
	if err := s.bookingRepo.UpdateStatus(ctx, bookingID, models.BookingStatusCancelled); err != nil {
		return nil, err
	}
	booking.Status = models.BookingStatusCancelled
	booking.CancelledAt = &now

	// 2. Refund user wallet
	if booking.AmountCents != nil && *booking.AmountCents > 0 {
		userAccount, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerUser, userID)
		if err == nil && userAccount != nil {
			_ = s.accountRepo.Credit(ctx, userAccount.ID, *booking.AmountCents)

			// Create refund payment record
			refundKey := fmt.Sprintf("refund_%s", bookingID)
			displayRef := fmt.Sprintf("RF-%05d", time.Now().UnixMilli()%100000)
			refundPayment := &models.Payment{
				ID:               uuid.New(),
				IdempotencyKey:   refundKey,
				AccountID:        userAccount.ID,
				Type:             models.PaymentTypeRefund,
				ReferenceID:      &bookingID,
				AmountCents:      *booking.AmountCents,
				Status:           models.PaymentStatusCompleted,
				DisplayReference: &displayRef,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			_ = s.paymentRepo.Create(ctx, refundPayment)
		}

		// 3. Debit host wallet for the net earning
		if booking.NetEarningCents != nil && *booking.NetEarningCents > 0 {
			evt, err := s.eventRepo.GetByID(ctx, booking.EventID)
			if err == nil && evt != nil {
				hostAccount, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, evt.HostID)
				if err == nil && hostAccount != nil {
					_ = s.accountRepo.Debit(ctx, hostAccount.ID, *booking.NetEarningCents)
					_ = s.payoutRepo.ClearPending(ctx, evt.HostID, *booking.NetEarningCents)
				}
			}
		}
	}

	s.dispatcher.Publish(event.BookingCancelled, booking)
	return booking, nil
}

func (s *bookingService) GetUserBookings(ctx context.Context, userID uuid.UUID) ([]*models.Booking, error) {
	return s.bookingRepo.ListByUserID(ctx, userID)
}
