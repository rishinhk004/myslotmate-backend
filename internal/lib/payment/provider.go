package payment

import "context"

// OrderRequest contains everything needed to create a payment order.
type OrderRequest struct {
	AmountCents int64  // in paise (INR cents)
	Currency    string // "INR"
	ReceiptID   string // our internal reference (idempotency)
	Notes       map[string]string
}

// OrderResponse is returned after creating an order with the provider.
type OrderResponse struct {
	OrderID     string // provider's order ID (e.g. order_xxxxx)
	AmountCents int64
	Currency    string
	Status      string // "created"
}

// VerifyRequest contains data the client sends after completing checkout.
type VerifyRequest struct {
	OrderID   string // razorpay_order_id
	PaymentID string // razorpay_payment_id
	Signature string // razorpay_signature
}

// Provider is the Strategy interface for payment collection (top-up, future direct pay).
// This is separate from the payout.Provider which is for sending money out.
type Provider interface {
	// CreateOrder creates a payment order. Client uses the returned order_id
	// to complete payment via Razorpay checkout SDK.
	CreateOrder(ctx context.Context, req OrderRequest) (*OrderResponse, error)

	// VerifyPaymentSignature verifies the client-side callback signature
	// (razorpay_order_id|razorpay_payment_id signed with key_secret).
	VerifyPaymentSignature(req VerifyRequest) bool

	// ValidateWebhookSignature verifies an incoming webhook payload.
	ValidateWebhookSignature(payload []byte, signature string) bool

	// GetKeyID returns the public key ID for the client-side checkout SDK.
	GetKeyID() string
}
