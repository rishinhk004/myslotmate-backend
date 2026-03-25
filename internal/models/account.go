package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BankDetails holds account/bank info (stored as JSONB in DB).
type BankDetails struct {
	AccountNumberMasked string `json:"account_number_masked,omitempty"`
	IFSC                string `json:"ifsc,omitempty"`
	UPIID               string `json:"upi_id,omitempty"`
	BeneficiaryName     string `json:"beneficiary_name,omitempty"`
}

// Account is the wallet and payment identity for a user or host (one per owner).
type Account struct {
	ID           uuid.UUID        `db:"id" json:"id"`
	OwnerType    AccountOwnerType `db:"owner_type" json:"owner_type"`
	OwnerID      uuid.UUID        `db:"owner_id" json:"owner_id"`
	BalanceCents int64            `db:"balance_cents" json:"balance_cents"`
	BankDetails  *json.RawMessage `db:"bank_details" json:"bank_details,omitempty"` // JSONB; use BankDetails when unmarshalling
	CreatedAt    time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at" json:"updated_at"`
}

// BankDetailsMap returns bank_details as a map for flexible use.
func (a *Account) BankDetailsMap() (map[string]interface{}, error) {
	var m map[string]interface{}
	if a.BankDetails == nil || len(*a.BankDetails) == 0 {
		return m, nil
	}
	err := json.Unmarshal(*a.BankDetails, &m)
	return m, err
}
