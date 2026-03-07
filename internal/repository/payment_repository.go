package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
)

// PaymentRepository provides payment (transaction ledger) data access.
type PaymentRepository interface {
	Create(ctx context.Context, payment *models.Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.PaymentStatus, lastError *string) error
	IncrementRetry(ctx context.Context, id uuid.UUID, lastError string) error
	ListByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*models.Payment, error)
	ListByTypeAndAccount(ctx context.Context, accountID uuid.UUID, paymentType models.PaymentType, limit, offset int) ([]*models.Payment, error)
}

type postgresPaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) PaymentRepository {
	return &postgresPaymentRepository{db: db}
}

func (r *postgresPaymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	query := `
		INSERT INTO payments (id, idempotency_key, account_id, type, reference_id, amount_cents, status, retry_count, last_error, payout_method_id, display_reference, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	if payment.ID == uuid.Nil {
		payment.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		payment.ID, payment.IdempotencyKey, payment.AccountID, payment.Type,
		payment.ReferenceID, payment.AmountCents, payment.Status, payment.RetryCount,
		payment.LastError, payment.PayoutMethodID, payment.DisplayReference,
		payment.CreatedAt, payment.UpdatedAt,
	)
	return err
}

func (r *postgresPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Payment, error) {
	p := &models.Payment{}
	query := `SELECT id, idempotency_key, account_id, type, reference_id, amount_cents, status, retry_count, last_error, payout_method_id, display_reference, created_at, updated_at FROM payments WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.IdempotencyKey, &p.AccountID, &p.Type, &p.ReferenceID,
		&p.AmountCents, &p.Status, &p.RetryCount, &p.LastError,
		&p.PayoutMethodID, &p.DisplayReference, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (r *postgresPaymentRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Payment, error) {
	p := &models.Payment{}
	query := `SELECT id, idempotency_key, account_id, type, reference_id, amount_cents, status, retry_count, last_error, payout_method_id, display_reference, created_at, updated_at FROM payments WHERE idempotency_key = $1`
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&p.ID, &p.IdempotencyKey, &p.AccountID, &p.Type, &p.ReferenceID,
		&p.AmountCents, &p.Status, &p.RetryCount, &p.LastError,
		&p.PayoutMethodID, &p.DisplayReference, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (r *postgresPaymentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.PaymentStatus, lastError *string) error {
	query := `UPDATE payments SET status = $1, last_error = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, lastError, id)
	return err
}

func (r *postgresPaymentRepository) IncrementRetry(ctx context.Context, id uuid.UUID, lastError string) error {
	query := `UPDATE payments SET retry_count = retry_count + 1, last_error = $1, status = 'failed' WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, lastError, id)
	return err
}

func (r *postgresPaymentRepository) ListByAccountID(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*models.Payment, error) {
	query := `SELECT id, idempotency_key, account_id, type, reference_id, amount_cents, status, retry_count, last_error, payout_method_id, display_reference, created_at, updated_at
		FROM payments WHERE account_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	return r.scanPayments(ctx, query, accountID, limit, offset)
}

func (r *postgresPaymentRepository) ListByTypeAndAccount(ctx context.Context, accountID uuid.UUID, paymentType models.PaymentType, limit, offset int) ([]*models.Payment, error) {
	query := `SELECT id, idempotency_key, account_id, type, reference_id, amount_cents, status, retry_count, last_error, payout_method_id, display_reference, created_at, updated_at
		FROM payments WHERE account_id = $1 AND type = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
	return r.scanPayments(ctx, query, accountID, paymentType, limit, offset)
}

func (r *postgresPaymentRepository) scanPayments(ctx context.Context, query string, args ...interface{}) ([]*models.Payment, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		p := &models.Payment{}
		if err := rows.Scan(
			&p.ID, &p.IdempotencyKey, &p.AccountID, &p.Type, &p.ReferenceID,
			&p.AmountCents, &p.Status, &p.RetryCount, &p.LastError,
			&p.PayoutMethodID, &p.DisplayReference, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, nil
}
