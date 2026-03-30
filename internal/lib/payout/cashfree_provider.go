package payout

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// CashfreeConfig holds credentials for Cashfree Payouts API.
type CashfreeConfig struct {
	BaseURL        string // CASHFREE_BASE_URL (default: https://payout-api.cashfree.com)
	ClientID       string // CASHFREE_CLIENT_ID
	ClientSecret   string // CASHFREE_CLIENT_SECRET
	WebhookSecret  string // CASHFREE_WEBHOOK_SECRET
	APIVersion     string // CASHFREE_API_VERSION
	PublicKeyPath  string // Path to RSA public key PEM file for 2FA signatures
	UseIPWhitelist bool   // Set to true if using IP whitelisting (simpler - no RSA needed)
}

// CashfreeProvider implements Provider using Cashfree Payouts API.
type CashfreeProvider struct {
	cfg    CashfreeConfig
	client *http.Client
}

const defaultCashfreeBaseURL = "https://payout-api.cashfree.com"

func NewCashfreeProvider(cfg CashfreeConfig) Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultCashfreeBaseURL
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = "2026-01-01"
	}

	return &CashfreeProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type cashfreeTransferReq struct {
	TransferID       string                     `json:"transfer_id"`
	TransferAmount   string                     `json:"transfer_amount"`
	TransferCurrency string                     `json:"transfer_currency"`
	TransferMode     string                     `json:"transfer_mode"`
	TransferRemarks  string                     `json:"transfer_remarks,omitempty"`
	Beneficiary      cashfreeBeneficiaryDetails `json:"beneficiary_details"`
}

type cashfreeBeneficiaryDetails struct {
	BeneficiaryName              string                               `json:"beneficiary_name,omitempty"`
	BeneficiaryInstrumentDetails cashfreeBeneficiaryInstrumentDetails `json:"beneficiary_instrument_details"`
}

type cashfreeBeneficiaryInstrumentDetails struct {
	BankAccountNumber string `json:"bank_account_number,omitempty"`
	BankIFSC          string `json:"bank_ifsc,omitempty"`
	VPA               string `json:"vpa,omitempty"`
}

func (p *CashfreeProvider) InitiateTransfer(ctx context.Context, req TransferRequest) (*TransferResponse, error) {
	payoutReq, err := buildCashfreeTransferRequest(req)
	if err != nil {
		fmt.Printf("[CASHFREE] InitiateTransfer error: %v\n", err)
		return nil, err
	}

	body, err := json.Marshal(payoutReq)
	if err != nil {
		fmt.Printf("[CASHFREE] Marshal error: %v\n", err)
		return nil, fmt.Errorf("failed to marshal cashfree payout request: %w", err)
	}

	fmt.Printf("[CASHFREE] InitiateTransfer request: paymentID=%s, amount=%d, method=%s, url=%s\n",
		req.PaymentID, req.AmountCents, req.MethodType, strings.TrimRight(p.cfg.BaseURL, "/")+"/payout/transfers")

	url := strings.TrimRight(p.cfg.BaseURL, "/") + "/payout/transfers"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		fmt.Printf("[CASHFREE] HTTP request creation error: %v\n", err)
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq, body, http.MethodPost, "/payout/transfers")

	// Debug: print all headers
	fmt.Printf("[CASHFREE] Request headers:\n")
	for key, values := range httpReq.Header {
		for _, value := range values {
			if key == "X-Client-Secret" {
				fmt.Printf("[CASHFREE]   %s: [REDACTED]\n", key)
			} else {
				fmt.Printf("[CASHFREE]   %s: %s\n", key, value)
			}
		}
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		fmt.Printf("[CASHFREE] HTTP call error: %v\n", err)
		return nil, fmt.Errorf("cashfree API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[CASHFREE] Response read error: %v\n", err)
		return nil, fmt.Errorf("failed to read cashfree response: %w", err)
	}

	fmt.Printf("[CASHFREE] Response received: httpStatus=%d, bodyLength=%d, body=%s\n",
		resp.StatusCode, len(respBody), string(respBody))

	if resp.StatusCode >= 400 {
		errorResp := &TransferResponse{
			Status: "failed",
			Error:  fmt.Sprintf("cashfree API error (HTTP %d): %s", resp.StatusCode, string(respBody)),
		}
		fmt.Printf("[CASHFREE] API error response: %v\n", errorResp)
		return errorResp, nil
	}

	parsed, err := parseCashfreeTransferResponse(respBody)
	if err != nil {
		fmt.Printf("[CASHFREE] Response parse error: %v\n", err)
		return nil, fmt.Errorf("failed to parse cashfree response: %w", err)
	}

	fmt.Printf("[CASHFREE] Response parsed: status=%s, providerRefID=%s, error=%s\n",
		parsed.Status, parsed.ProviderRefID, parsed.Error)

	if parsed.ProviderRefID == "" {
		parsed.ProviderRefID = req.PaymentID.String()
		fmt.Printf("[CASHFREE] No provider ref ID in response, using paymentID: %s\n", parsed.ProviderRefID)
	}

	fmt.Printf("[CASHFREE] InitiateTransfer completed: paymentID=%s, status=%s\n", req.PaymentID, parsed.Status)
	return parsed, nil
}

func (p *CashfreeProvider) CheckStatus(ctx context.Context, providerRefID string) (*TransferResponse, error) {
	if providerRefID == "" {
		return nil, fmt.Errorf("providerRefID is required")
	}

	path := "/payout/transfers/" + providerRefID
	url := strings.TrimRight(p.cfg.BaseURL, "/") + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	p.setHeaders(httpReq, nil, http.MethodGet, path)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cashfree API call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read cashfree response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cashfree API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	parsed, err := parseCashfreeTransferResponse(respBody)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cashfree response: %w", err)
	}

	if parsed.ProviderRefID == "" {
		parsed.ProviderRefID = providerRefID
	}

	return parsed, nil
}

// ValidateWebhookSignature verifies Cashfree webhook signature using HMAC-SHA256.
func (p *CashfreeProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	if p.cfg.WebhookSecret == "" {
		return false
	}

	sig := strings.TrimSpace(signature)
	if sig == "" {
		return false
	}
	sig = strings.TrimPrefix(sig, "sha256=")

	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write(payload)
	sum := mac.Sum(nil)

	expectedHex := hex.EncodeToString(sum)
	expectedB64 := base64.StdEncoding.EncodeToString(sum)
	expectedB64URL := base64.URLEncoding.EncodeToString(sum)

	if hmac.Equal([]byte(strings.ToLower(expectedHex)), []byte(strings.ToLower(sig))) {
		return true
	}
	if hmac.Equal([]byte(expectedB64), []byte(sig)) {
		return true
	}
	return hmac.Equal([]byte(expectedB64URL), []byte(sig))
}

func (p *CashfreeProvider) setHeaders(req *http.Request, body []byte, method string, path string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-client-id", p.cfg.ClientID)
	req.Header.Set("x-client-secret", p.cfg.ClientSecret)
	req.Header.Set("x-api-version", p.cfg.APIVersion)

	// Cashfree Payouts API requires 2FA signature in X-Cf-Signature header.
	// Two options:
	// 1. IP Whitelisting (simpler): No signature needed if server IP is whitelisted
	// 2. RSA Signatures (default): Sign with RSA public key from Cashfree dashboard

	if !p.cfg.UseIPWhitelist {
		// Mode: RSA signature required
		if len(body) > 0 {
			sig, err := p.generateRSASignature()
			if err != nil {
				fmt.Printf("[CASHFREE] RSA signature generation failed: %v (falling back to plain request)\n", err)
			} else if sig != "" {
				fmt.Printf("[CASHFREE] Generated RSA signature\n")
				// IMPORTANT: Header name is X-Cf-Signature (not x-signature)
				req.Header.Set("X-Cf-Signature", sig)
				fmt.Printf("[CASHFREE] Headers set with X-Cf-Signature header\n")
			}
		}
	} else {
		// Mode: IP Whitelisting
		fmt.Printf("[CASHFREE] Using IP whitelisting mode - no signature required\n")
	}
}

// generateRSASignature creates RSA-2048 signature for Cashfree 2FA.
// As per Cashfree docs: RSA encrypt "{clientID}.{timestamp}" with public key from dashboard,
// then Base64 encode the ciphertext.
func (p *CashfreeProvider) generateRSASignature() (string, error) {
	// If no public key configured, return empty (will use IP whitelist mode)
	if p.cfg.PublicKeyPath == "" {
		return "", fmt.Errorf("CASHFREE_PUBLIC_KEY_PATH not configured")
	}

	// Read PEM-encoded public key from file
	keyData, err := os.ReadFile(p.cfg.PublicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read RSA public key: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(keyData)
	if block == nil {
		return "", fmt.Errorf("failed to parse PEM block")
	}

	// Parse RSA public key
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("key is not RSA public key")
	}

	// Create signature string: clientID.timestamp
	timestamp := time.Now().Unix()
	message := fmt.Sprintf("%s.%d", p.cfg.ClientID, timestamp)

	// RSA encrypt with OAEPwith SHA-256
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte(message), nil)
	if err != nil {
		return "", fmt.Errorf("RSA encryption failed: %w", err)
	}

	// Base64 encode the ciphertext
	signature := base64.StdEncoding.EncodeToString(ciphertext)
	fmt.Printf("[CASHFREE] Signature calc: clientID=%s, timestamp=%d, signature_b64_len=%d\n", p.cfg.ClientID, timestamp, len(signature))
	return signature, nil
}

func buildCashfreeTransferRequest(req TransferRequest) (*cashfreeTransferReq, error) {
	beneficiaryName := strings.TrimSpace(req.BeneficiaryName)
	if beneficiaryName == "" {
		beneficiaryName = "MySlotMate Host"
	}

	payoutReq := &cashfreeTransferReq{
		TransferID:       req.PaymentID.String(),
		TransferAmount:   fmt.Sprintf("%.2f", float64(req.AmountCents)/100.0),
		TransferCurrency: "INR",
		TransferRemarks:  firstNonEmpty(req.IdempotencyKey, fmt.Sprintf("MySlotMate payout %s", req.PaymentID)),
		Beneficiary: cashfreeBeneficiaryDetails{
			BeneficiaryName: beneficiaryName,
		},
	}

	switch strings.ToLower(req.MethodType) {
	case "bank":
		if req.AccountNumber == "" || req.IFSC == "" {
			return nil, fmt.Errorf("bank payout requires account number and IFSC")
		}
		payoutReq.TransferMode = "banktransfer"
		payoutReq.Beneficiary.BeneficiaryInstrumentDetails = cashfreeBeneficiaryInstrumentDetails{
			BankAccountNumber: req.AccountNumber,
			BankIFSC:          req.IFSC,
		}

	case "upi":
		if req.UPIID == "" {
			return nil, fmt.Errorf("upi payout requires upi id")
		}
		payoutReq.TransferMode = "upi"
		payoutReq.Beneficiary.BeneficiaryInstrumentDetails = cashfreeBeneficiaryInstrumentDetails{
			VPA: req.UPIID,
		}

	default:
		return nil, fmt.Errorf("unsupported payout method type: %s", req.MethodType)
	}

	return payoutReq, nil
}

func parseCashfreeTransferResponse(respBody []byte) (*TransferResponse, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, err
	}

	providerRefID := firstNonEmpty(
		lookupString(payload, "cf_transfer_id"),
		lookupString(payload, "transfer_id"),
		lookupString(payload, "id"),
		lookupNestedString(payload, "data", "cf_transfer_id"),
		lookupNestedString(payload, "data", "transfer_id"),
		lookupNestedString(payload, "data", "id"),
		lookupNestedString(payload, "transfer", "cf_transfer_id"),
		lookupNestedString(payload, "transfer", "transfer_id"),
	)

	statusRaw := firstNonEmpty(
		lookupString(payload, "transfer_status"),
		lookupString(payload, "status"),
		lookupString(payload, "event"),
		lookupNestedString(payload, "data", "transfer_status"),
		lookupNestedString(payload, "data", "status"),
		lookupNestedString(payload, "data", "event"),
		lookupNestedString(payload, "transfer", "status"),
		lookupNestedString(payload, "transfer", "transfer_status"),
	)

	errMsg := firstNonEmpty(
		lookupString(payload, "message"),
		lookupString(payload, "reason"),
		lookupString(payload, "error"),
		lookupString(payload, "error_message"),
		lookupNestedString(payload, "data", "message"),
		lookupNestedString(payload, "data", "reason"),
		lookupNestedString(payload, "data", "error"),
		lookupNestedString(payload, "transfer", "reason"),
	)

	return &TransferResponse{
		ProviderRefID: providerRefID,
		Status:        mapCashfreeStatus(statusRaw),
		Error:         errMsg,
	}, nil
}

func mapCashfreeStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUCCESS", "COMPLETED", "PROCESSED", "TRANSFER_SUCCESS":
		return "completed"
	case "FAILED", "REJECTED", "CANCELLED", "CANCELED", "TRANSFER_FAILED":
		return "failed"
	case "REVERSED", "RETURNED", "TRANSFER_REVERSED":
		return "reversed"
	default:
		return "processing"
	}
}

func lookupString(values map[string]interface{}, key string) string {
	v, ok := values[key]
	if !ok {
		return ""
	}
	return toString(v)
}

func lookupNestedString(values map[string]interface{}, path ...string) string {
	if len(path) == 0 {
		return ""
	}

	var current interface{} = values
	for _, part := range path {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = asMap[part]
		if !ok {
			return ""
		}
	}

	return toString(current)
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
