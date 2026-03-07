package models

import (
	"time"

	"github.com/google/uuid"
)

// SavedExperience is a user's bookmark/favorite of an event (experience).
type SavedExperience struct {
	ID      uuid.UUID `db:"id" json:"id"`
	UserID  uuid.UUID `db:"user_id" json:"user_id"`
	EventID uuid.UUID `db:"event_id" json:"event_id"`
	SavedAt time.Time `db:"saved_at" json:"saved_at"`
}
