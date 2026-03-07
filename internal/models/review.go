package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Review is a user-written review for an event with a star rating.
type Review struct {
	ID             uuid.UUID      `db:"id" json:"id"`
	EventID        uuid.UUID      `db:"event_id" json:"event_id"`
	UserID         uuid.UUID      `db:"user_id" json:"user_id"`
	Rating         int            `db:"rating" json:"rating"` // 1-5 stars
	Name           *string        `db:"name" json:"name,omitempty"`
	Description    string         `db:"description" json:"description"`
	PhotoURLs      pq.StringArray `db:"photo_urls" json:"photo_urls"` // uploaded review photos
	Reply          []string       `db:"reply" json:"reply"`           // PostgreSQL text[]
	SentimentScore *float64       `db:"sentiment_score" json:"sentiment_score,omitempty"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at"`
}
