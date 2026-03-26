package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// PlatformSettings stores config like platform fee percentages.
type PlatformSettings struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	Key       string          `db:"key" json:"key"`
	Value     json.RawMessage `db:"value" json:"value"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// PlatformFeeConfig is the structure for the "platform_fee" key.
type PlatformFeeConfig struct {
	HostPercentage     int `json:"host_percentage"`     // e.g. 70
	PlatformPercentage int `json:"platform_percentage"` // e.g. 30
}
