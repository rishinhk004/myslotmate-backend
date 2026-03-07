package repository

import (
	"context"
	"database/sql"
	"myslotmate-backend/internal/models"

	"github.com/google/uuid"
)

type BookingRepository interface {
	Create(ctx context.Context, booking *models.Booking) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Booking, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Booking, error)
	GetTotalBookedQuantity(ctx context.Context, eventID uuid.UUID) (int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.BookingStatus) error
}

type postgresBookingRepository struct {
	db *sql.DB
}

func NewBookingRepository(db *sql.DB) BookingRepository {
	return &postgresBookingRepository{db: db}
}

func (r *postgresBookingRepository) Create(ctx context.Context, booking *models.Booking) error {
	query := `
		INSERT INTO bookings (id, event_id, user_id, quantity, status, payment_id, idempotency_key, amount_cents, service_fee_cents, net_earning_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	if booking.ID == uuid.Nil {
		booking.ID = uuid.New()
	}
	_, err := r.db.ExecContext(ctx, query,
		booking.ID, booking.EventID, booking.UserID, booking.Quantity, booking.Status, booking.PaymentID, booking.IdempotencyKey, booking.AmountCents, booking.ServiceFeeCents, booking.NetEarningCents, booking.CreatedAt, booking.UpdatedAt,
	)
	return err
}

func (r *postgresBookingRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	b := &models.Booking{}
	query := `SELECT id, event_id, user_id, quantity, status, payment_id, idempotency_key, amount_cents, service_fee_cents, net_earning_cents, created_at, updated_at, cancelled_at FROM bookings WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.EventID, &b.UserID, &b.Quantity, &b.Status, &b.PaymentID, &b.IdempotencyKey, &b.AmountCents, &b.ServiceFeeCents, &b.NetEarningCents, &b.CreatedAt, &b.UpdatedAt, &b.CancelledAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return b, nil
}

func (r *postgresBookingRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*models.Booking, error) {
	query := `SELECT id, event_id, user_id, quantity, status, payment_id, idempotency_key, amount_cents, service_fee_cents, net_earning_cents, created_at, updated_at, cancelled_at FROM bookings WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*models.Booking
	for rows.Next() {
		b := &models.Booking{}
		if err := rows.Scan(
			&b.ID, &b.EventID, &b.UserID, &b.Quantity, &b.Status, &b.PaymentID, &b.IdempotencyKey, &b.AmountCents, &b.ServiceFeeCents, &b.NetEarningCents, &b.CreatedAt, &b.UpdatedAt, &b.CancelledAt,
		); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}
	return bookings, nil
}

func (r *postgresBookingRepository) GetTotalBookedQuantity(ctx context.Context, eventID uuid.UUID) (int, error) {
	query := `SELECT COALESCE(SUM(quantity), 0) FROM bookings WHERE event_id = $1 AND status IN ('pending', 'confirmed')`
	var total int
	err := r.db.QueryRowContext(ctx, query, eventID).Scan(&total)
	return total, err
}

func (r *postgresBookingRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.BookingStatus) error {
	query := `UPDATE bookings SET status = $1 WHERE id = $2`
	if status == models.BookingStatusCancelled {
		query = `UPDATE bookings SET status = $1, cancelled_at = NOW() WHERE id = $2`
	}
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}
