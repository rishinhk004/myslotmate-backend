package models

import (
	"time"

	"github.com/google/uuid"
)

// Review is a user-written review for an event (AI can set sentiment_score).
type Review struct {
	ID             uuid.UUID   `db:"id" json:"id"`
	EventID        uuid.UUID   `db:"event_id" json:"event_id"`
	UserID         uuid.UUID   `db:"user_id" json:"user_id"`
	Name           *string     `db:"name" json:"name,omitempty"`
	Description    string      `db:"description" json:"description"`
	Reply          []string    `db:"reply" json:"reply"` // PostgreSQL text[]
	SentimentScore *float64    `db:"sentiment_score" json:"sentiment_score,omitempty"`
	CreatedAt      time.Time   `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time   `db:"updated_at" json:"updated_at"`
}
