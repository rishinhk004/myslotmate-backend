package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SupportTicketMessage is a single message in a support thread.
type SupportTicketMessage struct {
	Sender    string    `json:"sender"`    // "user" | "support"
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// SupportTicket is a conversation between user/host and support.
type SupportTicket struct {
	ID        uuid.UUID             `db:"id" json:"id"`
	UserID    uuid.UUID             `db:"user_id" json:"user_id"`
	Subject   string                `db:"subject" json:"subject"`
	Messages  json.RawMessage       `db:"messages" json:"messages"` // JSONB array of SupportTicketMessage
	Status    SupportTicketStatus   `db:"status" json:"status"`
	CreatedAt time.Time             `db:"created_at" json:"created_at"`
	UpdatedAt time.Time             `db:"updated_at" json:"updated_at"`
}
