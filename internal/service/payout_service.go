package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"myslotmate-backend/internal/lib/event"
	"myslotmate-backend/internal/lib/payout"
	"myslotmate-backend/internal/models"
	"myslotmate-backend/internal/repository"

	"github.com/google/uuid"
)

// PayoutService handles payout methods, withdrawal requests, earnings queries, and webhooks.
type PayoutService interface {
	// Payout Methods
	AddPayoutMethod(ctx context.Context, hostID uuid.UUID, req AddPayoutMethodRequest) (*models.PayoutMethod, error)
	ListPayoutMethods(ctx context.Context, hostID uuid.UUID) ([]*models.PayoutMethod, error)
	SetPrimaryMethod(ctx context.Context, hostID uuid.UUID, methodID uuid.UUID) error
	DeletePayoutMethod(ctx context.Context, hostID uuid.UUID, methodID uuid.UUID) error

	// Withdrawals
	RequestWithdrawal(ctx context.Context, hostID uuid.UUID, req WithdrawalRequest) (*models.Payment, error)

	// Earnings Dashboard
	GetEarningsSummary(ctx context.Context, hostID uuid.UUID) (*EarningsSummary, error)

	// Payment History
	GetPayoutHistory(ctx context.Context, hostID uuid.UUID, limit, offset int) ([]*models.Payment, error)

	// Webhook
	HandlePayoutWebhook(ctx context.Context, paymentID uuid.UUID, status string, providerError string) error
}

// ── Request / Response types ────────────────────────────────────────────────

type AddPayoutMethodRequest struct {
	Type            models.PayoutMethodType
	BankName        *string
	AccountType     *string
	AccountNumber   *string // will be encrypted + last 4 stored
	IFSC            *string
	BeneficiaryName *string
	UPIID           *string
}

type WithdrawalRequest struct {
	AmountCents    int64
	PayoutMethodID *uuid.UUID // if nil, use primary
	IdempotencyKey string
}

type EarningsSummary struct {
	AvailableBalanceCents int64                     `json:"available_balance_cents"`
	TotalEarningsCents    int64                     `json:"total_earnings_cents"`
	PendingClearanceCents int64                     `json:"pending_clearance_cents"`
	EstimatedClearanceAt  *time.Time                `json:"estimated_clearance_at,omitempty"`
	PlatformFee           *models.PlatformFeeConfig `json:"platform_fee"`
}

// ── Implementation ──────────────────────────────────────────────────────────

type payoutService struct {
	payoutRepo  repository.PayoutRepository
	accountRepo repository.AccountRepository
	paymentRepo repository.PaymentRepository
	hostRepo    repository.HostRepository
	provider    payout.Provider
	dispatcher  *event.Dispatcher
}

func NewPayoutService(
	pr repository.PayoutRepository,
	ar repository.AccountRepository,
	pmr repository.PaymentRepository,
	hr repository.HostRepository,
	provider payout.Provider,
	d *event.Dispatcher,
) PayoutService {
	return &payoutService{
		payoutRepo:  pr,
		accountRepo: ar,
		paymentRepo: pmr,
		hostRepo:    hr,
		provider:    provider,
		dispatcher:  d,
	}
}

// ── Payout Methods ──────────────────────────────────────────────────────────

func (s *payoutService) AddPayoutMethod(ctx context.Context, hostID uuid.UUID, req AddPayoutMethodRequest) (*models.PayoutMethod, error) {
	// Determine if this is the first method (auto-set as primary)
	existing, err := s.payoutRepo.ListPayoutMethodsByHostID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	isPrimary := len(existing) == 0

	// Mask + encrypt account number for bank type
	var lastFour *string
	var encrypted *string
	if req.Type == models.PayoutMethodBank && req.AccountNumber != nil {
		num := *req.AccountNumber
		if len(num) >= 4 {
			l4 := num[len(num)-4:]
			lastFour = &l4
		}
		// In production, encrypt with KMS/vault. For now, store raw (marked as encrypted field).
		encrypted = req.AccountNumber
	}

	pm := &models.PayoutMethod{
		ID:                     uuid.New(),
		HostID:                 hostID,
		Type:                   req.Type,
		BankName:               req.BankName,
		AccountType:            req.AccountType,
		LastFourDigits:         lastFour,
		AccountNumberEncrypted: encrypted,
		IFSC:                   req.IFSC,
		BeneficiaryName:        req.BeneficiaryName,
		UPIID:                  req.UPIID,
		IsVerified:             false, // needs verification flow
		IsPrimary:              isPrimary,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	if err := s.payoutRepo.CreatePayoutMethod(ctx, pm); err != nil {
		return nil, err
	}

	return pm, nil
}

func (s *payoutService) ListPayoutMethods(ctx context.Context, hostID uuid.UUID) ([]*models.PayoutMethod, error) {
	return s.payoutRepo.ListPayoutMethodsByHostID(ctx, hostID)
}

func (s *payoutService) SetPrimaryMethod(ctx context.Context, hostID uuid.UUID, methodID uuid.UUID) error {
	pm, err := s.payoutRepo.GetPayoutMethodByID(ctx, methodID)
	if err != nil {
		return err
	}
	if pm == nil {
		return errors.New("payout method not found")
	}
	if pm.HostID != hostID {
		return errors.New("payout method does not belong to this host")
	}
	return s.payoutRepo.SetPrimary(ctx, hostID, methodID)
}

func (s *payoutService) DeletePayoutMethod(ctx context.Context, hostID uuid.UUID, methodID uuid.UUID) error {
	pm, err := s.payoutRepo.GetPayoutMethodByID(ctx, methodID)
	if err != nil {
		return err
	}
	if pm == nil {
		return errors.New("payout method not found")
	}
	if pm.HostID != hostID {
		return errors.New("payout method does not belong to this host")
	}
	if pm.IsPrimary {
		return errors.New("cannot delete the primary payout method; set another as primary first")
	}
	return s.payoutRepo.DeletePayoutMethod(ctx, methodID)
}

// ── Withdrawal ──────────────────────────────────────────────────────────────

func (s *payoutService) RequestWithdrawal(ctx context.Context, hostID uuid.UUID, req WithdrawalRequest) (*models.Payment, error) {
	if req.AmountCents <= 0 {
		return nil, errors.New("withdrawal amount must be positive")
	}

	// 1. Check idempotency — return existing payment if already processed
	if req.IdempotencyKey != "" {
		existing, err := s.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	// 2. Get host and check fraud flags
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}
	if host == nil {
		return nil, errors.New("host not found")
	}

	flagged, err := s.payoutRepo.HasActiveFraudFlag(ctx, host.UserID)
	if err != nil {
		return nil, err
	}
	if flagged {
		return nil, errors.New("account is blocked due to suspicious activity")
	}

	// 3. Get host account
	account, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, hostID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("host account not found")
	}

	// 4. Determine payout method
	var payoutMethod *models.PayoutMethod
	if req.PayoutMethodID != nil {
		payoutMethod, err = s.payoutRepo.GetPayoutMethodByID(ctx, *req.PayoutMethodID)
		if err != nil {
			return nil, err
		}
	} else {
		payoutMethod, err = s.payoutRepo.GetPrimaryPayoutMethod(ctx, hostID)
		if err != nil {
			return nil, err
		}
	}
	if payoutMethod == nil {
		return nil, errors.New("no payout method found; please add a bank account or UPI")
	}
	if payoutMethod.HostID != hostID {
		return nil, errors.New("payout method does not belong to this host")
	}
	if !payoutMethod.IsVerified {
		return nil, errors.New("payout method is not verified yet")
	}

	// 5. Debit wallet (optimistic — credit back on failure)
	if err := s.accountRepo.Debit(ctx, account.ID, req.AmountCents); err != nil {
		if errors.Is(err, repository.ErrInsufficientBalance) {
			return nil, errors.New("insufficient balance for withdrawal")
		}
		return nil, err
	}

	// 6. Create payment record
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("payout_%s_%d", hostID, time.Now().UnixNano())
	}
	displayRef := fmt.Sprintf("TXN-%05d", time.Now().UnixMilli()%100000)

	payment := &models.Payment{
		ID:               uuid.New(),
		IdempotencyKey:   idempotencyKey,
		AccountID:        account.ID,
		Type:             models.PaymentTypePayout,
		AmountCents:      req.AmountCents,
		Status:           models.PaymentStatusPending,
		RetryCount:       0,
		PayoutMethodID:   &payoutMethod.ID,
		DisplayReference: &displayRef,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		// Credit back on failure to create payment record
		_ = s.accountRepo.Credit(ctx, account.ID, req.AmountCents)
		return nil, err
	}

	// 7. Call external provider (async in production, sync with mock)
	transferReq := payout.TransferRequest{
		PaymentID:      payment.ID,
		AmountCents:    req.AmountCents,
		MethodType:     string(payoutMethod.Type),
		IdempotencyKey: idempotencyKey,
	}
	if payoutMethod.BeneficiaryName != nil {
		transferReq.BeneficiaryName = *payoutMethod.BeneficiaryName
	}
	if payoutMethod.Type == models.PayoutMethodBank {
		if payoutMethod.AccountNumberEncrypted != nil {
			transferReq.AccountNumber = *payoutMethod.AccountNumberEncrypted
		}
		if payoutMethod.IFSC != nil {
			transferReq.IFSC = *payoutMethod.IFSC
		}
		if payoutMethod.BankName != nil {
			transferReq.BankName = *payoutMethod.BankName
		}
	} else if payoutMethod.Type == models.PayoutMethodUPI {
		if payoutMethod.UPIID != nil {
			transferReq.UPIID = *payoutMethod.UPIID
		}
	}

	// Update status to processing
	_ = s.paymentRepo.UpdateStatus(ctx, payment.ID, models.PaymentStatusProcessing, nil)
	payment.Status = models.PaymentStatusProcessing

	resp, err := s.provider.InitiateTransfer(ctx, transferReq)
	if err != nil {
		// Provider call failed — mark payment as failed, credit wallet back
		errMsg := err.Error()
		_ = s.paymentRepo.IncrementRetry(ctx, payment.ID, errMsg)
		_ = s.accountRepo.Credit(ctx, account.ID, req.AmountCents)
		payment.Status = models.PaymentStatusFailed
		payment.LastError = &errMsg
		return payment, nil
	}

	// 8. Handle provider response
	if resp.Status == "completed" {
		_ = s.paymentRepo.UpdateStatus(ctx, payment.ID, models.PaymentStatusCompleted, nil)
		payment.Status = models.PaymentStatusCompleted
		s.dispatcher.Publish(event.PayoutCompleted, payment)
	} else if resp.Status == "failed" {
		_ = s.paymentRepo.IncrementRetry(ctx, payment.ID, resp.Error)
		_ = s.accountRepo.Credit(ctx, account.ID, req.AmountCents)
		payment.Status = models.PaymentStatusFailed
		payment.LastError = &resp.Error
	}
	// If "processing", the webhook will finalize

	return payment, nil
}

// ── Earnings Dashboard ──────────────────────────────────────────────────────

func (s *payoutService) GetEarningsSummary(ctx context.Context, hostID uuid.UUID) (*EarningsSummary, error) {
	// Get host account balance
	account, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, hostID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("host account not found")
	}

	// Get host earnings aggregate
	earnings, err := s.payoutRepo.GetHostEarnings(ctx, hostID)
	if err != nil {
		return nil, err
	}

	// Get platform fee config
	feeConfig, err := s.payoutRepo.GetPlatformFeeConfig(ctx)
	if err != nil {
		return nil, err
	}

	summary := &EarningsSummary{
		AvailableBalanceCents: account.BalanceCents,
		PlatformFee:           feeConfig,
	}

	if earnings != nil {
		summary.TotalEarningsCents = earnings.TotalEarningsCents
		summary.PendingClearanceCents = earnings.PendingClearanceCents
		summary.EstimatedClearanceAt = earnings.EstimatedClearanceAt
	}

	return summary, nil
}

// ── Payment History ─────────────────────────────────────────────────────────

func (s *payoutService) GetPayoutHistory(ctx context.Context, hostID uuid.UUID, limit, offset int) ([]*models.Payment, error) {
	account, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, hostID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.New("host account not found")
	}

	if limit <= 0 {
		limit = 20
	}

	return s.paymentRepo.ListByTypeAndAccount(ctx, account.ID, models.PaymentTypePayout, limit, offset)
}

// ── Webhook Handler ─────────────────────────────────────────────────────────

func (s *payoutService) HandlePayoutWebhook(ctx context.Context, paymentID uuid.UUID, status string, providerError string) error {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if payment == nil {
		return errors.New("payment not found")
	}
	if payment.Type != models.PaymentTypePayout {
		return errors.New("payment is not a payout")
	}

	switch status {
	case "completed":
		return s.paymentRepo.UpdateStatus(ctx, paymentID, models.PaymentStatusCompleted, nil)

	case "failed":
		// Credit the amount back to host wallet
		if err := s.accountRepo.Credit(ctx, payment.AccountID, payment.AmountCents); err != nil {
			return fmt.Errorf("failed to credit wallet on payout failure: %w", err)
		}
		return s.paymentRepo.IncrementRetry(ctx, paymentID, providerError)

	case "reversed":
		// Credit the amount back to host wallet
		if err := s.accountRepo.Credit(ctx, payment.AccountID, payment.AmountCents); err != nil {
			return fmt.Errorf("failed to credit wallet on payout reversal: %w", err)
		}
		return s.paymentRepo.UpdateStatus(ctx, paymentID, models.PaymentStatusReversed, &providerError)

	default:
		return fmt.Errorf("unknown payout status: %s", status)
	}
}
