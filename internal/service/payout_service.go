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
	fmt.Printf("[PAYOUT] AddPayoutMethod: hostID=%s, type=%s\n", hostID, req.Type)

	// Determine if this is the first method (auto-set as primary)
	existing, err := s.payoutRepo.ListPayoutMethodsByHostID(ctx, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] AddPayoutMethod: list methods error: %v\n", err)
		return nil, err
	}
	isPrimary := len(existing) == 0
	fmt.Printf("[PAYOUT] AddPayoutMethod: existing methods=%d, isPrimary=%v\n", len(existing), isPrimary)

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
		IsVerified:             true, // auto-verified by default
		IsPrimary:              isPrimary,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	fmt.Printf("[PAYOUT] AddPayoutMethod: creating method - methodID=%s, verified=true, primary=%v\n", pm.ID, isPrimary)
	if err := s.payoutRepo.CreatePayoutMethod(ctx, pm); err != nil {
		fmt.Printf("[PAYOUT] AddPayoutMethod: create error: %v\n", err)
		return nil, err
	}

	fmt.Printf("[PAYOUT] AddPayoutMethod: method created successfully\n")
	return pm, nil
}

func (s *payoutService) ListPayoutMethods(ctx context.Context, hostID uuid.UUID) ([]*models.PayoutMethod, error) {
	fmt.Printf("[PAYOUT] ListPayoutMethods: hostID=%s\n", hostID)
	methods, err := s.payoutRepo.ListPayoutMethodsByHostID(ctx, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] ListPayoutMethods: error: %v\n", err)
		return nil, err
	}
	fmt.Printf("[PAYOUT] ListPayoutMethods: found %d methods\n", len(methods))
	return methods, nil
}

func (s *payoutService) SetPrimaryMethod(ctx context.Context, hostID uuid.UUID, methodID uuid.UUID) error {
	fmt.Printf("[PAYOUT] SetPrimaryMethod: hostID=%s, methodID=%s\n", hostID, methodID)

	pm, err := s.payoutRepo.GetPayoutMethodByID(ctx, methodID)
	if err != nil {
		fmt.Printf("[PAYOUT] SetPrimaryMethod: fetch error: %v\n", err)
		return err
	}
	if pm == nil {
		fmt.Printf("[PAYOUT] SetPrimaryMethod: method not found\n")
		return errors.New("payout method not found")
	}
	if pm.HostID != hostID {
		fmt.Printf("[PAYOUT] SetPrimaryMethod: method does not belong to host\n")
		return errors.New("payout method does not belong to this host")
	}

	fmt.Printf("[PAYOUT] SetPrimaryMethod: setting as primary\n")
	err = s.payoutRepo.SetPrimary(ctx, hostID, methodID)
	if err != nil {
		fmt.Printf("[PAYOUT] SetPrimaryMethod: set primary error: %v\n", err)
		return err
	}
	fmt.Printf("[PAYOUT] SetPrimaryMethod: successfully set as primary\n")
	return nil
}

func (s *payoutService) DeletePayoutMethod(ctx context.Context, hostID uuid.UUID, methodID uuid.UUID) error {
	fmt.Printf("[PAYOUT] DeletePayoutMethod: hostID=%s, methodID=%s\n", hostID, methodID)

	pm, err := s.payoutRepo.GetPayoutMethodByID(ctx, methodID)
	if err != nil {
		fmt.Printf("[PAYOUT] DeletePayoutMethod: fetch error: %v\n", err)
		return err
	}
	if pm == nil {
		fmt.Printf("[PAYOUT] DeletePayoutMethod: method not found\n")
		return errors.New("payout method not found")
	}
	if pm.HostID != hostID {
		fmt.Printf("[PAYOUT] DeletePayoutMethod: method does not belong to host\n")
		return errors.New("payout method does not belong to this host")
	}
	if pm.IsPrimary {
		fmt.Printf("[PAYOUT] DeletePayoutMethod: cannot delete primary method\n")
		return errors.New("cannot delete the primary payout method; set another as primary first")
	}

	fmt.Printf("[PAYOUT] DeletePayoutMethod: deleting method\n")
	err = s.payoutRepo.DeletePayoutMethod(ctx, methodID)
	if err != nil {
		fmt.Printf("[PAYOUT] DeletePayoutMethod: delete error: %v\n", err)
		return err
	}
	fmt.Printf("[PAYOUT] DeletePayoutMethod: deleted successfully\n")
	return nil
}

// ── Withdrawal ──────────────────────────────────────────────────────────────

func (s *payoutService) RequestWithdrawal(ctx context.Context, hostID uuid.UUID, req WithdrawalRequest) (*models.Payment, error) {
	fmt.Printf("[PAYOUT] RequestWithdrawal started: hostID=%s, amount=%d, idempotencyKey=%s\n", hostID, req.AmountCents, req.IdempotencyKey)

	if req.AmountCents <= 0 {
		fmt.Printf("[PAYOUT] RequestWithdrawal failed: invalid amount %d\n", req.AmountCents)
		return nil, errors.New("withdrawal amount must be positive")
	}

	// 1. Check idempotency — return existing payment if already processed
	if req.IdempotencyKey != "" {
		fmt.Printf("[PAYOUT] Checking idempotency for key: %s\n", req.IdempotencyKey)
		existing, err := s.paymentRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			fmt.Printf("[PAYOUT] Idempotency check error: %v\n", err)
			return nil, err
		}
		if existing != nil {
			fmt.Printf("[PAYOUT] Idempotent request - returning existing payment: %s with status %s\n", existing.ID, existing.Status)
			return existing, nil
		}
	}

	// 2. Get host and check fraud flags
	fmt.Printf("[PAYOUT] Fetching host: %s\n", hostID)
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] Host fetch error: %v\n", err)
		return nil, err
	}
	if host == nil {
		fmt.Printf("[PAYOUT] Host not found: %s\n", hostID)
		return nil, errors.New("host not found")
	}

	fmt.Printf("[PAYOUT] Checking fraud flags for user: %s\n", host.UserID)
	flagged, err := s.payoutRepo.HasActiveFraudFlag(ctx, host.UserID)
	if err != nil {
		fmt.Printf("[PAYOUT] Fraud flag check error: %v\n", err)
		return nil, err
	}
	if flagged {
		fmt.Printf("[PAYOUT] Account flagged for fraud: userID=%s\n", host.UserID)
		return nil, errors.New("account is blocked due to suspicious activity")
	}

	// 3. Get host account
	fmt.Printf("[PAYOUT] Fetching host account\n")
	account, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] Account fetch error: %v\n", err)
		return nil, err
	}
	if account == nil {
		fmt.Printf("[PAYOUT] Host account not found\n")
		return nil, errors.New("host account not found")
	}
	fmt.Printf("[PAYOUT] Account found: accountID=%s, currentBalance=%d\n", account.ID, account.BalanceCents)

	// 4. Determine payout method
	fmt.Printf("[PAYOUT] Selecting payout method\n")
	var payoutMethod *models.PayoutMethod
	if req.PayoutMethodID != nil {
		fmt.Printf("[PAYOUT] Using specified method: %s\n", *req.PayoutMethodID)
		payoutMethod, err = s.payoutRepo.GetPayoutMethodByID(ctx, *req.PayoutMethodID)
		if err != nil {
			fmt.Printf("[PAYOUT] Payout method fetch error: %v\n", err)
			return nil, err
		}
	} else {
		fmt.Printf("[PAYOUT] Using primary payout method\n")
		payoutMethod, err = s.payoutRepo.GetPrimaryPayoutMethod(ctx, hostID)
		if err != nil {
			fmt.Printf("[PAYOUT] Primary payout method fetch error: %v\n", err)
			return nil, err
		}
	}
	if payoutMethod == nil {
		fmt.Printf("[PAYOUT] No payout method available\n")
		return nil, errors.New("no payout method found; please add a bank account or UPI")
	}
	if payoutMethod.HostID != hostID {
		fmt.Printf("[PAYOUT] Payout method does not belong to host\n")
		return nil, errors.New("payout method does not belong to this host")
	}
	if !payoutMethod.IsVerified {
		fmt.Printf("[PAYOUT] Payout method not verified: %s\n", payoutMethod.ID)
		return nil, errors.New("payout method is not verified yet")
	}
	fmt.Printf("[PAYOUT] Payout method selected: methodID=%s, type=%s, verified=true\n", payoutMethod.ID, payoutMethod.Type)

	// 5. Debit wallet (optimistic — credit back on failure)
	fmt.Printf("[PAYOUT] Debiting wallet: accountID=%s, amount=%d\n", account.ID, req.AmountCents)
	if err := s.accountRepo.Debit(ctx, account.ID, req.AmountCents); err != nil {
		fmt.Printf("[PAYOUT] Debit failed: %v\n", err)
		if errors.Is(err, repository.ErrInsufficientBalance) {
			return nil, errors.New("insufficient balance for withdrawal")
		}
		return nil, err
	}
	fmt.Printf("[PAYOUT] Wallet debited successfully\n")

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

	fmt.Printf("[PAYOUT] Creating payment record: paymentID=%s, displayRef=%s\n", payment.ID, displayRef)
	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		fmt.Printf("[PAYOUT] Payment record creation failed: %v (rolling back debit)\n", err)
		// Credit back on failure to create payment record
		_ = s.accountRepo.Credit(ctx, account.ID, req.AmountCents)
		return nil, err
	}
	fmt.Printf("[PAYOUT] Payment record created successfully\n")

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
	fmt.Printf("[PAYOUT] Updating payment status to PROCESSING\n")
	_ = s.paymentRepo.UpdateStatus(ctx, payment.ID, models.PaymentStatusProcessing, nil)
	payment.Status = models.PaymentStatusProcessing

	fmt.Printf("[PAYOUT] Calling provider: InitiateTransfer with amount=%d, method=%s\n", req.AmountCents, transferReq.MethodType)
	resp, err := s.provider.InitiateTransfer(ctx, transferReq)
	if err != nil {
		fmt.Printf("[PAYOUT] Provider call failed: %v (rolling back debit)\n", err)
		// Provider call failed — mark payment as failed, credit wallet back
		errMsg := err.Error()
		_ = s.paymentRepo.IncrementRetry(ctx, payment.ID, errMsg)
		_ = s.accountRepo.Credit(ctx, account.ID, req.AmountCents)
		payment.Status = models.PaymentStatusFailed
		payment.LastError = &errMsg
		fmt.Printf("[PAYOUT] Payment marked as FAILED and wallet reconciled\n")
		return payment, nil
	}

	fmt.Printf("[PAYOUT] Provider response: status=%s, providerRefID=%s, error=%s\n", resp.Status, resp.ProviderRefID, resp.Error)

	// 8. Handle provider response
	if resp.Status == "completed" {
		fmt.Printf("[PAYOUT] Payment completed successfully by provider\n")
		_ = s.paymentRepo.UpdateStatus(ctx, payment.ID, models.PaymentStatusCompleted, nil)
		payment.Status = models.PaymentStatusCompleted
		s.dispatcher.Publish(event.PayoutCompleted, payment)
		fmt.Printf("[PAYOUT] Payment finalized: paymentID=%s, amount=%d, status=COMPLETED\n", payment.ID, req.AmountCents)
	} else if resp.Status == "failed" {
		fmt.Printf("[PAYOUT] Payment failed by provider: %s (rolling back debit)\n", resp.Error)
		_ = s.paymentRepo.IncrementRetry(ctx, payment.ID, resp.Error)
		_ = s.accountRepo.Credit(ctx, account.ID, req.AmountCents)
		payment.Status = models.PaymentStatusFailed
		payment.LastError = &resp.Error
		fmt.Printf("[PAYOUT] Payment marked as FAILED and wallet reconciled\n")
	} else {
		fmt.Printf("[PAYOUT] Payment status=%s - waiting for webhook update\n", resp.Status)
	}
	// If "processing", the webhook will finalize

	fmt.Printf("[PAYOUT] RequestWithdrawal completed: paymentID=%s, status=%s\n", payment.ID, payment.Status)
	return payment, nil
}

// ── Earnings Dashboard ──────────────────────────────────────────────────────

func (s *payoutService) GetEarningsSummary(ctx context.Context, hostID uuid.UUID) (*EarningsSummary, error) {
	fmt.Printf("[PAYOUT] GetEarningsSummary: hostID=%s\n", hostID)

	// Get host account balance
	account, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] GetEarningsSummary: account fetch error: %v\n", err)
		return nil, err
	}
	if account == nil {
		fmt.Printf("[PAYOUT] GetEarningsSummary: host account not found\n")
		return nil, errors.New("host account not found")
	}
	fmt.Printf("[PAYOUT] GetEarningsSummary: account found - accountID=%s, balance=%d\n", account.ID, account.BalanceCents)

	// Get host earnings aggregate
	earnings, err := s.payoutRepo.GetHostEarnings(ctx, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] GetEarningsSummary: earnings fetch error: %v\n", err)
		return nil, err
	}

	// Get platform fee config
	feeConfig, err := s.payoutRepo.GetPlatformFeeConfig(ctx)
	if err != nil {
		fmt.Printf("[PAYOUT] GetEarningsSummary: fee config fetch error: %v\n", err)
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
		fmt.Printf("[PAYOUT] GetEarningsSummary: earnings - total=%d, pending=%d, clearanceAt=%v\n",
			earnings.TotalEarningsCents, earnings.PendingClearanceCents, earnings.EstimatedClearanceAt)
	} else {
		fmt.Printf("[PAYOUT] GetEarningsSummary: no earnings record found\n")
	}

	fmt.Printf("[PAYOUT] GetEarningsSummary: returning summary - available=%d, total=%d, pending=%d\n",
		summary.AvailableBalanceCents, summary.TotalEarningsCents, summary.PendingClearanceCents)
	return summary, nil
}

// ── Payment History ─────────────────────────────────────────────────────────

func (s *payoutService) GetPayoutHistory(ctx context.Context, hostID uuid.UUID, limit, offset int) ([]*models.Payment, error) {
	fmt.Printf("[PAYOUT] GetPayoutHistory: hostID=%s, limit=%d, offset=%d\n", hostID, limit, offset)

	account, err := s.accountRepo.GetByOwner(ctx, models.AccountOwnerHost, hostID)
	if err != nil {
		fmt.Printf("[PAYOUT] GetPayoutHistory: account fetch error: %v\n", err)
		return nil, err
	}
	if account == nil {
		fmt.Printf("[PAYOUT] GetPayoutHistory: host account not found\n")
		return nil, errors.New("host account not found")
	}

	if limit <= 0 {
		limit = 20
	}

	payments, err := s.paymentRepo.ListByTypeAndAccount(ctx, account.ID, models.PaymentTypePayout, limit, offset)
	if err != nil {
		fmt.Printf("[PAYOUT] GetPayoutHistory: list error: %v\n", err)
		return nil, err
	}

	fmt.Printf("[PAYOUT] GetPayoutHistory: found %d payments\n", len(payments))
	for i, p := range payments {
		fmt.Printf("[PAYOUT]   [%d] paymentID=%s, amount=%d, status=%s, createdAt=%v\n",
			i+1, p.ID, p.AmountCents, p.Status, p.CreatedAt)
	}

	return payments, nil
}

// ── Webhook Handler ─────────────────────────────────────────────────────────

func (s *payoutService) HandlePayoutWebhook(ctx context.Context, paymentID uuid.UUID, status string, providerError string) error {
	fmt.Printf("[PAYOUT] HandlePayoutWebhook: paymentID=%s, status=%s, error=%s\n", paymentID, status, providerError)

	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		fmt.Printf("[PAYOUT] Webhook: payment fetch error: %v\n", err)
		return err
	}
	if payment == nil {
		fmt.Printf("[PAYOUT] Webhook: payment not found: %s\n", paymentID)
		return errors.New("payment not found")
	}
	if payment.Type != models.PaymentTypePayout {
		fmt.Printf("[PAYOUT] Webhook: payment is not a payout: %s\n", paymentID)
		return errors.New("payment is not a payout")
	}

	fmt.Printf("[PAYOUT] Webhook: processing payout update - current status=%s, new status=%s\n", payment.Status, status)

	switch status {
	case "completed":
		fmt.Printf("[PAYOUT] Webhook: marking payment as completed\n")
		return s.paymentRepo.UpdateStatus(ctx, paymentID, models.PaymentStatusCompleted, nil)

	case "failed":
		fmt.Printf("[PAYOUT] Webhook: payment failed, crediting wallet: amount=%d\n", payment.AmountCents)
		// Credit the amount back to host wallet
		if err := s.accountRepo.Credit(ctx, payment.AccountID, payment.AmountCents); err != nil {
			fmt.Printf("[PAYOUT] Webhook: wallet credit failed: %v\n", err)
			return fmt.Errorf("failed to credit wallet on payout failure: %w", err)
		}
		return s.paymentRepo.IncrementRetry(ctx, paymentID, providerError)

	case "reversed":
		fmt.Printf("[PAYOUT] Webhook: payment reversed, crediting wallet: amount=%d\n", payment.AmountCents)
		// Credit the amount back to host wallet
		if err := s.accountRepo.Credit(ctx, payment.AccountID, payment.AmountCents); err != nil {
			fmt.Printf("[PAYOUT] Webhook: wallet credit failed: %v\n", err)
			return fmt.Errorf("failed to credit wallet on payout reversal: %w", err)
		}
		return s.paymentRepo.UpdateStatus(ctx, paymentID, models.PaymentStatusReversed, &providerError)

	default:
		fmt.Printf("[PAYOUT] Webhook: unknown payout status: %s\n", status)
		return fmt.Errorf("unknown payout status: %s", status)
	}
}
