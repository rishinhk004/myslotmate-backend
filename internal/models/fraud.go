package models

import (
	"time"

	"github.com/google/uuid"
)

// FraudFlag marks a user as blocked or suspicious (booking spike, payment abuse, etc.).
type FraudFlag struct {
	ID          uuid.UUID    `db:"id" json:"id"`
	UserID      uuid.UUID    `db:"user_id" json:"user_id"`
	Type        FraudFlagType `db:"type" json:"type"`
	Reason      *string      `db:"reason" json:"reason,omitempty"`
	BlockedAt   time.Time    `db:"blocked_at" json:"blocked_at"`
	BlockedUntil *time.Time  `db:"blocked_until" json:"blocked_until,omitempty"`
	IsActive    bool        `db:"is_active" json:"is_active"`
}
