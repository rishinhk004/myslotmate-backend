package models

import (
	"time"

	"github.com/google/uuid"
)

// Host is a verified user who can create events, see analytics, and manage payouts.
type Host struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	UserID       uuid.UUID  `db:"user_id" json:"user_id"`
	Name         string     `db:"name" json:"name"`
	PhnNumber   string     `db:"phn_number" json:"phn_number"`
	AccountID    *uuid.UUID `db:"account_id" json:"account_id,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}
