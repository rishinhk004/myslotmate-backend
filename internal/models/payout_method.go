package models

import (
	"time"

	"github.com/google/uuid"
)

// PayoutMethod is a bank account or UPI ID for receiving payouts.
type PayoutMethod struct {
	ID                     uuid.UUID        `db:"id" json:"id"`
	HostID                 *uuid.UUID       `db:"host_id" json:"host_id,omitempty"`
	Type                   PayoutMethodType `db:"type" json:"type"`
	BankName               *string          `db:"bank_name" json:"bank_name,omitempty"`
	AccountType            *string          `db:"account_type" json:"account_type,omitempty"` // checking, savings
	LastFourDigits         *string          `db:"last_four_digits" json:"last_four_digits,omitempty"`
	AccountNumberEncrypted *string          `db:"account_number_encrypted" json:"-"` // never expose
	IFSC                   *string          `db:"ifsc" json:"ifsc,omitempty"`
	BeneficiaryName        *string          `db:"beneficiary_name" json:"beneficiary_name,omitempty"`
	UPIID                  *string          `db:"upi_id" json:"upi_id,omitempty"`
	CashfreeBeID           *string          `db:"cashfree_be_id" json:"cashfree_be_id,omitempty"` // Cashfree beneficiary ID
	IsVerified             bool             `db:"is_verified" json:"is_verified"`
	IsPrimary              bool             `db:"is_primary" json:"is_primary"`
	CreatedAt              time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time        `db:"updated_at" json:"updated_at"`
}
