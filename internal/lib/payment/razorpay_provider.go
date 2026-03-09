package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RazorpayConfig holds credentials for the Razorpay Standard (Payment Gateway) API.
type RazorpayConfig struct {
	KeyID         string // RAZORPAY_KEY_ID (same key used for payouts)
	KeySecret     string // RAZORPAY_KEY_SECRET
	WebhookSecret string // RAZORPAY_PAYMENT_WEBHOOK_SECRET (can differ from payout webhook secret)
}

// RazorpayProvider implements Provider using Razorpay Standard (Orders + Payments API).
// API docs: https://razorpay.com/docs/api/orders
type RazorpayProvider struct {
	cfg    RazorpayConfig
	client *http.Client
}

const razorpayBaseURL = "https://api.razorpay.com/v1"

func NewRazorpayProvider(cfg RazorpayConfig) Provider {
	return &RazorpayProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ── Razorpay API structures ─────────────────────────────────────────────────

type razorpayCreateOrderReq struct {
	Amount   int64             `json:"amount"`   // in paise
	Currency string            `json:"currency"` // "INR"
	Receipt  string            `json:"receipt"`
	Notes    map[string]string `json:"notes,omitempty"`
}

type razorpayOrderResp struct {
	ID       string `json:"id"`       // order_xxxxx
	Entity   string `json:"entity"`   // "order"
	Amount   int64  `json:"amount"`   // in paise
	Currency string `json:"currency"` // "INR"
	Receipt  string `json:"receipt"`
	Status   string `json:"status"` // created, attempted, paid
	Error    *struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"error,omitempty"`
}

// ── Provider interface implementation ───────────────────────────────────────

func (p *RazorpayProvider) CreateOrder(ctx context.Context, req OrderRequest) (*OrderResponse, error) {
	currency := req.Currency
	if currency == "" {
		currency = "INR"
	}

	orderReq := razorpayCreateOrderReq{
		Amount:   req.AmountCents, // Razorpay expects paise = INR cents
		Currency: currency,
		Receipt:  req.ReceiptID,
		Notes:    req.Notes,
	}

	body, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, razorpayBaseURL+"/orders", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.SetBasicAuth(p.cfg.KeyID, p.cfg.KeySecret)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("razorpay API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read razorpay response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("razorpay API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var orderResp razorpayOrderResp
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to parse razorpay response: %w", err)
	}

	return &OrderResponse{
		OrderID:     orderResp.ID,
		AmountCents: orderResp.Amount,
		Currency:    orderResp.Currency,
		Status:      orderResp.Status,
	}, nil
}

// VerifyPaymentSignature verifies the client-side Razorpay checkout callback.
// Razorpay signs: razorpay_order_id + "|" + razorpay_payment_id using HMAC-SHA256 with key_secret.
func (p *RazorpayProvider) VerifyPaymentSignature(req VerifyRequest) bool {
	message := req.OrderID + "|" + req.PaymentID
	mac := hmac.New(sha256.New, []byte(p.cfg.KeySecret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(req.Signature))
}

// ValidateWebhookSignature verifies Razorpay webhook using HMAC-SHA256.
func (p *RazorpayProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	if p.cfg.WebhookSecret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// GetKeyID returns the public Razorpay key ID for the client checkout SDK.
func (p *RazorpayProvider) GetKeyID() string {
	return p.cfg.KeyID
}
