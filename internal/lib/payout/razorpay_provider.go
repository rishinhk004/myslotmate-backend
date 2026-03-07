package payout

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

// RazorpayConfig holds credentials for the Razorpay Payouts API.
type RazorpayConfig struct {
	KeyID         string // RAZORPAY_KEY_ID
	KeySecret     string // RAZORPAY_KEY_SECRET
	AccountNumber string // RAZORPAY_ACCOUNT_NUMBER (your RazorpayX linked account)
	WebhookSecret string // RAZORPAY_WEBHOOK_SECRET (for signature verification)
}

// RazorpayProvider implements Provider using Razorpay Payouts API (RazorpayX).
// API docs: https://razorpay.com/docs/api/x/payouts
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

// ── Razorpay API request/response structures ────────────────────────────────

// razorpayCreatePayoutReq matches POST /v1/payouts
type razorpayCreatePayoutReq struct {
	AccountNumber string                  `json:"account_number"`
	FundAccountID string                  `json:"fund_account_id,omitempty"`
	FundAccount   *razorpayFundAccountReq `json:"fund_account,omitempty"` // inline create
	Amount        int64                   `json:"amount"`                 // in paise
	Currency      string                  `json:"currency"`
	Mode          string                  `json:"mode"` // NEFT, RTGS, IMPS, UPI
	Purpose       string                  `json:"purpose"`
	ReferenceID   string                  `json:"reference_id,omitempty"` // idempotency
	Narration     string                  `json:"narration,omitempty"`
}

type razorpayFundAccountReq struct {
	AccountType string                    `json:"account_type"` // bank_account or vpa
	BankAccount *razorpayBankAccountReq   `json:"bank_account,omitempty"`
	VPA         *razorpayVPAReq           `json:"vpa,omitempty"`
	Contact     razorpayContactReq        `json:"contact"`
}

type razorpayBankAccountReq struct {
	Name          string `json:"name"`
	IFSC          string `json:"ifsc"`
	AccountNumber string `json:"account_number"`
}

type razorpayVPAReq struct {
	Address string `json:"address"` // UPI ID e.g. user@okhdfc
}

type razorpayContactReq struct {
	Name string `json:"name"`
	Type string `json:"type"` // vendor, customer, employee, self
}

// razorpayPayoutResp is the response from POST /v1/payouts and GET /v1/payouts/:id
type razorpayPayoutResp struct {
	ID            string `json:"id"`     // pout_xxxxx
	Entity        string `json:"entity"` // "payout"
	FundAccountID string `json:"fund_account_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	Status        string `json:"status"` // queued, processing, processed, reversed, cancelled, failed
	Mode          string `json:"mode"`
	Purpose       string `json:"purpose"`
	ReferenceID   string `json:"reference_id"`
	Narration     string `json:"narration"`
	FailureReason string `json:"failure_reason,omitempty"`
	Error         *struct {
		Code        string `json:"code"`
		Description string `json:"description"`
		Source      string `json:"source"`
		Reason      string `json:"reason"`
	} `json:"error,omitempty"`
}

// ── Provider interface implementation ───────────────────────────────────────

func (p *RazorpayProvider) InitiateTransfer(ctx context.Context, req TransferRequest) (*TransferResponse, error) {
	// Build the payout request with inline fund account creation
	payoutReq := razorpayCreatePayoutReq{
		AccountNumber: p.cfg.AccountNumber,
		Amount:        req.AmountCents, // Razorpay expects amount in paise (= cents for INR)
		Currency:      "INR",
		Purpose:       "payout",
		ReferenceID:   req.IdempotencyKey,
		Narration:     fmt.Sprintf("MySlotMate Payout %s", req.PaymentID),
	}

	fundAccount := &razorpayFundAccountReq{
		Contact: razorpayContactReq{
			Name: req.BeneficiaryName,
			Type: "vendor",
		},
	}

	if req.MethodType == "bank" {
		payoutReq.Mode = "IMPS" // fastest for amounts up to 5L; use NEFT/RTGS for larger
		fundAccount.AccountType = "bank_account"
		fundAccount.BankAccount = &razorpayBankAccountReq{
			Name:          req.BeneficiaryName,
			IFSC:          req.IFSC,
			AccountNumber: req.AccountNumber,
		}
	} else if req.MethodType == "upi" {
		payoutReq.Mode = "UPI"
		fundAccount.AccountType = "vpa"
		fundAccount.VPA = &razorpayVPAReq{
			Address: req.UPIID,
		}
	} else {
		return nil, fmt.Errorf("unsupported payout method type: %s", req.MethodType)
	}

	payoutReq.FundAccount = fundAccount

	// Make API call
	body, err := json.Marshal(payoutReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payout request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, razorpayBaseURL+"/payouts", bytes.NewReader(body))
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
		return &TransferResponse{
			Status: "failed",
			Error:  fmt.Sprintf("razorpay API error (HTTP %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	var payoutResp razorpayPayoutResp
	if err := json.Unmarshal(respBody, &payoutResp); err != nil {
		return nil, fmt.Errorf("failed to parse razorpay response: %w", err)
	}

	return &TransferResponse{
		ProviderRefID: payoutResp.ID,
		Status:        mapRazorpayStatus(payoutResp.Status),
		Error:         payoutResp.FailureReason,
	}, nil
}

func (p *RazorpayProvider) CheckStatus(ctx context.Context, providerRefID string) (*TransferResponse, error) {
	url := fmt.Sprintf("%s/payouts/%s", razorpayBaseURL, providerRefID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.SetBasicAuth(p.cfg.KeyID, p.cfg.KeySecret)

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

	var payoutResp razorpayPayoutResp
	if err := json.Unmarshal(respBody, &payoutResp); err != nil {
		return nil, fmt.Errorf("failed to parse razorpay response: %w", err)
	}

	return &TransferResponse{
		ProviderRefID: payoutResp.ID,
		Status:        mapRazorpayStatus(payoutResp.Status),
		Error:         payoutResp.FailureReason,
	}, nil
}

// ValidateWebhookSignature verifies Razorpay webhook using HMAC-SHA256.
// Razorpay sends the signature in the X-Razorpay-Signature header.
func (p *RazorpayProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	if p.cfg.WebhookSecret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// mapRazorpayStatus converts Razorpay payout statuses to our internal statuses.
// Razorpay statuses: queued, processing, processed, reversed, cancelled, failed
func mapRazorpayStatus(rzpStatus string) string {
	switch rzpStatus {
	case "processed":
		return "completed"
	case "queued", "processing":
		return "processing"
	case "reversed", "cancelled":
		return "reversed"
	case "failed":
		return "failed"
	default:
		return "processing"
	}
}
