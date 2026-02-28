package models

import (
	"time"

	"github.com/google/uuid"
)

// Event is created by a host and has a capacity to prevent overbooking.
type Event struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	HostID       uuid.UUID  `db:"host_id" json:"host_id"`
	Name         string     `db:"name" json:"name"`
	Time         time.Time  `db:"time" json:"time"`
	EndTime      *time.Time `db:"end_time" json:"end_time,omitempty"`
	Capacity     int        `db:"capacity" json:"capacity"`
	AISuggestion *string    `db:"ai_suggestion" json:"ai_suggestion,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}
