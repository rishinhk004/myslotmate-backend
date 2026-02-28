package models

import (
	"time"

	"github.com/google/uuid"
)

// HostEarnings stores aggregate earnings per host for the Earnings & Payouts dashboard.
type HostEarnings struct {
	ID                    uuid.UUID  `db:"id" json:"id"`
	HostID                uuid.UUID  `db:"host_id" json:"host_id"`
	TotalEarningsCents    int64      `db:"total_earnings_cents" json:"total_earnings_cents"`
	PendingClearanceCents int64      `db:"pending_clearance_cents" json:"pending_clearance_cents"`
	EstimatedClearanceAt  *time.Time `db:"estimated_clearance_at" json:"estimated_clearance_at,omitempty"`
	CreatedAt             time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at" json:"updated_at"`
}
