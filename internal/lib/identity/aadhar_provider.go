package identity

import "context"

// AadharVerificationResult contains the outcome of a successful verification
type AadharVerificationResult struct {
	Success     bool
	Name        string // Name fetched from Aadhar, to cross-check with User provided name
	ReferenceID string // Provider's reference ID
}

// AadharProvider is an abstraction for 3rd party Aadhar KYC services (e.g. Setu, HyperVerge, Karza)
// Strategy Pattern: Allows swapping providers easily.
type AadharProvider interface {
	// InitiateVerification sends an OTP to the Aadhar-linked mobile number
	InitiateVerification(ctx context.Context, aadharNumber string) (transactionID string, err error)

	// VerifyOTP validates the OTP and completes the KYC
	VerifyOTP(ctx context.Context, transactionID string, otp string) (*AadharVerificationResult, error)
}
