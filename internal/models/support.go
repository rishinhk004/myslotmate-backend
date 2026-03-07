package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// SupportTicketMessage is a single message in a support thread.
type SupportTicketMessage struct {
	Sender    string    `json:"sender"` // "user" | "support"
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// SupportTicket is a conversation between user/host and support.
type SupportTicket struct {
	ID             uuid.UUID           `db:"id" json:"id"`
	UserID         uuid.UUID           `db:"user_id" json:"user_id"`
	Category       SupportCategory     `db:"category" json:"category"`                           // report_participant, technical_support, policy_help
	ReportedUserID *uuid.UUID          `db:"reported_user_id" json:"reported_user_id,omitempty"` // for report_participant
	Subject        string              `db:"subject" json:"subject"`
	Messages       json.RawMessage     `db:"messages" json:"messages"` // JSONB array of SupportTicketMessage
	Status         SupportTicketStatus `db:"status" json:"status"`

	// Report-specific fields (from Figma "Report a Participant" screen)
	EventID      *uuid.UUID     `db:"event_id" json:"event_id,omitempty"`           // Select Experience
	SessionDate  *time.Time     `db:"session_date" json:"session_date,omitempty"`   // Session Date
	ReportReason *ReportReason  `db:"report_reason" json:"report_reason,omitempty"` // verbal_harassment, safety_concern, etc.
	EvidenceURLs pq.StringArray `db:"evidence_urls" json:"evidence_urls,omitempty"` // uploaded file URLs
	IsUrgent     bool           `db:"is_urgent" json:"is_urgent"`                   // Urgent Safety Concern toggle

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
