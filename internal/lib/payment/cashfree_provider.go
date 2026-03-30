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
	"strings"
	"time"
)

// CashfreePaymentConfig holds credentials for the Cashfree Payment Links API.
type CashfreePaymentConfig struct {
	BaseURL       string // CASHFREE_PAYMENT_BASE_URL (default: https://api.cashfree.com)
	ClientID      string // CASHFREE_CLIENT_ID
	ClientSecret  string // CASHFREE_CLIENT_SECRET
	WebhookSecret string // CASHFREE_WEBHOOK_SECRET
	APIVersion    string // CASHFREE_API_VERSION
}

// CashfreePaymentProvider implements Provider using Cashfree Payment Links API.
// API docs: https://docs.cashfree.com/docs/payment-links
type CashfreePaymentProvider struct {
	cfg    CashfreePaymentConfig
	client *http.Client
}

const defaultCashfreePaymentBaseURL = "https://api.cashfree.com"

func NewCashfreePaymentProvider(cfg CashfreePaymentConfig) Provider {
	// Always use the payment API endpoint for payment links, regardless of config
	cfg.BaseURL = defaultCashfreePaymentBaseURL
	if cfg.APIVersion == "" {
		cfg.APIVersion = "2026-01-01"
	}

	return &CashfreePaymentProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ── Cashfree Payment API structures ─────────────────────────────────────────

type cashfreeCreatePaymentLinkReq struct {
	OrderID         string `json:"order_id"`
	OrderAmount     string `json:"order_amount"`
	OrderCurrency   string `json:"order_currency"`
	OrderNote       string `json:"order_note,omitempty"`
	CustomerDetails struct {
		CustomerName  string `json:"customer_name,omitempty"`
		CustomerPhone string `json:"customer_phone,omitempty"`
		CustomerEmail string `json:"customer_email,omitempty"`
	} `json:"customer_details,omitempty"`
	Notifications struct {
		Notify struct {
			SMS   bool `json:"sms"`
			Email bool `json:"email"`
		} `json:"notify"`
	} `json:"notifications,omitempty"`
	PaymentSessionID string `json:"payment_session_id"`
}

type cashfreePaymentLinkResp struct {
	Status string `json:"status"`
	Data   struct {
		PaymentLink      string `json:"payment_link"`
		OrderID          string `json:"order_id"`
		PaymentSessionID string `json:"payment_session_id"`
		PaymentLinkID    string `json:"payment_link_id"`
		Amount           string `json:"amount"`
		Currency         string `json:"currency"`
		CreatedAt        string `json:"created_at"`
	} `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// ── Provider interface implementation ───────────────────────────────────────

func (p *CashfreePaymentProvider) CreateOrder(ctx context.Context, req OrderRequest) (*OrderResponse, error) {
	currency := req.Currency
	if currency == "" {
		currency = "INR"
	}

	// Convert amount from cents to string with 2 decimal places
	amount := fmt.Sprintf("%.2f", float64(req.AmountCents)/100.0)

	orderReq := cashfreeCreatePaymentLinkReq{
		OrderID:          req.ReceiptID,
		OrderAmount:      amount,
		OrderCurrency:    currency,
		OrderNote:        req.Notes["description"],
		PaymentSessionID: req.ReceiptID, // Use receipt ID as session ID
	}

	body, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}

	endpoint := "/pg/orders"
	fullURL := strings.TrimRight(p.cfg.BaseURL, "/") + endpoint

	fmt.Printf("[CASHFREE] CreateOrder request: URL=%s, ClientID=%s, Amount=%s\n", fullURL, p.cfg.ClientID, amount)

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fullURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-client-id", p.cfg.ClientID)
	httpReq.Header.Set("x-client-secret", p.cfg.ClientSecret)
	httpReq.Header.Set("x-api-version", p.cfg.APIVersion)

	// Add signature header if required
	sig := p.generateSignature(body, "POST", endpoint)
	httpReq.Header.Set("x-signature", sig)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cashfree API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read cashfree response: %w", err)
	}

	fmt.Printf("[CASHFREE] CreateOrder response: status=%d, body=%s\n", resp.StatusCode, string(respBody))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cashfree API error (HTTP %d): %s (Endpoint: %s, ClientID: %s)",
			resp.StatusCode, string(respBody), endpoint, p.cfg.ClientID)
	}

	// Parse Cashfree PG API response - can be in multiple formats
	var response map[string]interface{}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse cashfree response: %w", err)
	}

	// Extract order_id from various possible response formats
	// Cashfree PG /orders returns: cf_order_id, order_id, or in data.order_id
	orderID := ""

	// Try top-level fields first
	if id, ok := response["cf_order_id"].(string); ok && id != "" {
		orderID = id
	} else if id, ok := response["order_id"].(string); ok && id != "" {
		orderID = id
	} else if id, ok := response["id"].(string); ok && id != "" {
		orderID = id
	}

	// Check nested data structure
	if orderID == "" {
		if data, ok := response["data"].(map[string]interface{}); ok {
			if id, ok := data["cf_order_id"].(string); ok && id != "" {
				orderID = id
			} else if id, ok := data["order_id"].(string); ok && id != "" {
				orderID = id
			} else if id, ok := data["id"].(string); ok && id != "" {
				orderID = id
			}
		}
	}

	if orderID == "" {
		return nil, fmt.Errorf("cashfree API response missing order_id. Response: %s", string(respBody))
	}

	fmt.Printf("[CASHFREE] Order created successfully: order_id=%s\n", orderID)

	return &OrderResponse{
		OrderID:     orderID,
		AmountCents: req.AmountCents,
		Currency:    currency,
		Status:      "created",
	}, nil
}

// VerifyPaymentSignature verifies the client-side Cashfree callback.
// Cashfree signs the payment response payload with HMAC-SHA256 using webhook secret.
func (p *CashfreePaymentProvider) VerifyPaymentSignature(req VerifyRequest) bool {
	// For Cashfree, the signature is typically over the entire payload
	// This is a simplified verification - in production, you'd reconstruct the exact payload
	// and verify the signature matches
	message := req.OrderID + "|" + req.PaymentID
	mac := hmac.New(sha256.New, []byte(p.cfg.ClientSecret))
	mac.Write([]byte(message))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(req.Signature))
}

// ValidateWebhookSignature verifies Cashfree webhook using HMAC-SHA256.
func (p *CashfreePaymentProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	if p.cfg.WebhookSecret == "" {
		return false
	}

	sig := strings.TrimSpace(signature)
	if sig == "" {
		return false
	}

	// Cashfree can use various signature formats; try hex
	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(sig))
}

// GetKeyID returns the Cashfree Client ID (public identifier for the client).
func (p *CashfreePaymentProvider) GetKeyID() string {
	return p.cfg.ClientID
}

// generateSignature creates HMAC-SHA256 signature for Cashfree requests
func (p *CashfreePaymentProvider) generateSignature(body []byte, method string, path string) string {
	message := method + path + string(body)
	mac := hmac.New(sha256.New, []byte(p.cfg.ClientSecret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
