package models

import (
	"time"

	"github.com/google/uuid"
)

// InboxMessage is a host broadcast for an event (inbox = updates per event).
type InboxMessage struct {
	ID        uuid.UUID `db:"id" json:"id"`
	EventID   uuid.UUID `db:"event_id" json:"event_id"`
	HostID    uuid.UUID `db:"host_id" json:"host_id"`
	Message   string    `db:"message" json:"message"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
