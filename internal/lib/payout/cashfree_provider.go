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
	"strings"
	"time"
)

// CashfreeConfig holds credentials for Cashfree Payouts API.
type CashfreeConfig struct {
	BaseURL       string // CASHFREE_BASE_URL (default: https://payout-api.cashfree.com)
	ClientID      string // CASHFREE_CLIENT_ID
	ClientSecret  string // CASHFREE_CLIENT_SECRET
	PublicKey     string // CASHFREE_PUBLIC_KEY (PEM-encoded RSA public key for signature generation)
	WebhookSecret string // CASHFREE_WEBHOOK_SECRET
	APIVersion    string // CASHFREE_API_VERSION
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
		cfg.APIVersion = "2024-01-01"
	}

	provider := &CashfreeProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return provider
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
	if err := p.setHeaders(httpReq); err != nil {
		fmt.Printf("[CASHFREE] Header setup error: %v\n", err)
		return nil, fmt.Errorf("failed to set headers: %w", err)
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
	if err := p.setHeaders(httpReq); err != nil {
		return nil, fmt.Errorf("failed to set headers: %w", err)
	}

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

func (p *CashfreeProvider) setHeaders(req *http.Request) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-client-id", p.cfg.ClientID)
	req.Header.Set("x-client-secret", p.cfg.ClientSecret)
	req.Header.Set("x-api-version", p.cfg.APIVersion)

	// Generate RSA signature: ClientId.timestamp encrypted with public key
	sig, err := p.generateRSASignature()
	if err != nil {
		fmt.Printf("[CASHFREE] RSA signature generation error: %v\n", err)
		return err
	}

	req.Header.Set("x-cf-signature", sig)
	fmt.Printf("[CASHFREE] Headers set with x-cf-signature header\n")
	return nil
}

// generateRSASignature creates RSA signature for Cashfree Payouts API 2FA
// Format: ClientId.timestamp encrypted with RSA public key, then base64 encoded
func (p *CashfreeProvider) generateRSASignature() (string, error) {
	fmt.Printf("[CASHFREE_PROVIDER] generateRSASignature called, public key length: %d\n", len(p.cfg.PublicKey))

	if p.cfg.PublicKey == "" {
		fmt.Printf("[CASHFREE_PROVIDER] ERROR: Public key is empty!\n")
		return "", fmt.Errorf("public key is not configured")
	}

	// Create message: clientId.timestamp (must be exact format)
	timestamp := time.Now().Unix()
	messageStr := fmt.Sprintf("%s.%d", p.cfg.ClientID, timestamp)
	messageBytes := []byte(messageStr)
	fmt.Printf("[CASHFREE_PROVIDER] RSA message string: %s\n", messageStr)
	fmt.Printf("[CASHFREE_PROVIDER] RSA message bytes length: %d\n", len(messageBytes))

	// Parse PEM-encoded public key
	pubKeyBlock, restPEM := pem.Decode([]byte(p.cfg.PublicKey))
	if pubKeyBlock == nil {
		fmt.Printf("[CASHFREE_PROVIDER] ERROR: Failed to decode PEM block from key\n")
		return "", fmt.Errorf("failed to decode public key PEM block")
	}

	fmt.Printf("[CASHFREE_PROVIDER] PEM block decoded successfully (type=%s, remaining=%d bytes)\n", pubKeyBlock.Type, len(restPEM))

	// Parse the public key
	pubKey, err := x509.ParsePKIXPublicKey(pubKeyBlock.Bytes)
	if err != nil {
		fmt.Printf("[CASHFREE_PROVIDER] ERROR: Failed to parse PKIX public key: %v\n", err)
		return "", fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		fmt.Printf("[CASHFREE_PROVIDER] ERROR: Public key is not RSA type\n")
		return "", fmt.Errorf("public key is not RSA")
	}

	fmt.Printf("[CASHFREE_PROVIDER] RSA public key parsed successfully (keySize=%d bits)\n", rsaPubKey.N.BitLen())

	// Encrypt using RSA PKCS#1 v1.5 (Cashfree requirement for 2FA)
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPubKey, messageBytes)
	if err != nil {
		fmt.Printf("[CASHFREE_PROVIDER] ERROR: RSA PKCS#1 v1.5 encryption failed: %v\n", err)
		return "", fmt.Errorf("failed to encrypt with RSA: %w", err)
	}

	fmt.Printf("[CASHFREE_PROVIDER] RSA encryption successful (ciphertext length=%d bytes)\n", len(ciphertext))

	// Base64 encode the ciphertext
	signature := base64.StdEncoding.EncodeToString(ciphertext)
	fmt.Printf("[CASHFREE_PROVIDER] Base64 signature length: %d chars\n", len(signature))
	fmt.Printf("[CASHFREE_PROVIDER] X-Cf-Signature header value (first 100 chars): %s...\n", signature[:minInt(100, len(signature))])
	fmt.Printf("[CASHFREE_PROVIDER] RSA signature generated successfully (PKCS#1 v1.5): clientID=%s, timestamp=%d\n", p.cfg.ClientID, timestamp)
	return signature, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
