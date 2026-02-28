package models

import (
	"time"

	"github.com/google/uuid"
)

// Booking links a user to an event with quantity and status.
type Booking struct {
	ID               uuid.UUID      `db:"id" json:"id"`
	EventID          uuid.UUID      `db:"event_id" json:"event_id"`
	UserID           uuid.UUID      `db:"user_id" json:"user_id"`
	Quantity         int            `db:"quantity" json:"quantity"`
	Status           BookingStatus  `db:"status" json:"status"`
	PaymentID        *uuid.UUID     `db:"payment_id" json:"payment_id,omitempty"`
	IdempotencyKey   *string        `db:"idempotency_key" json:"idempotency_key,omitempty"`
	AmountCents      *int64         `db:"amount_cents" json:"amount_cents,omitempty"`           // total booking value
	ServiceFeeCents  *int64         `db:"service_fee_cents" json:"service_fee_cents,omitempty"`  // platform fee (15%)
	NetEarningCents  *int64         `db:"net_earning_cents" json:"net_earning_cents,omitempty"`   // host net (85%)
	CreatedAt        time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at" json:"updated_at"`
	CancelledAt      *time.Time     `db:"cancelled_at" json:"cancelled_at,omitempty"`
}
