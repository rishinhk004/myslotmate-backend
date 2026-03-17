package models

import (
	"time"

	"github.com/google/uuid"
)

// InboxMessage is a message in an event session thread (host, guest, or system).
type InboxMessage struct {
	ID            uuid.UUID         `db:"id" json:"id"`
	EventID       uuid.UUID         `db:"event_id" json:"event_id"`
	HostID        uuid.UUID         `db:"host_id" json:"host_id"`
	SenderType    MessageSenderType `db:"sender_type" json:"sender_type"`       // system, host, guest
	SenderID      *uuid.UUID        `db:"sender_id" json:"sender_id,omitempty"` // nil for system messages
	Message       string            `db:"message" json:"message"`
	AttachmentURL *string           `db:"attachment_url" json:"attachment_url,omitempty"`
	IsRead        bool              `db:"is_read" json:"is_read"`
	CreatedAt     time.Time         `db:"created_at" json:"created_at"`
}
