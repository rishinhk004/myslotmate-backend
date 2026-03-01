package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// SetuConfig holds API keys for Setu
type SetuConfig struct {
	BaseURL           string
	ClientID          string
	ClientSecret      string
	ProductInstanceID string // Some Setu APIs require this
}

// SetuAadharProvider implements AadharProvider using Setu's OKYC API
type SetuAadharProvider struct {
	client *http.Client
	config SetuConfig
}

func NewSetuAadharProvider(cfg SetuConfig) AadharProvider {
	return &SetuAadharProvider{
		client: &http.Client{Timeout: 30 * time.Second},
		config: cfg,
	}
}

type setuInitRequest struct {
	AadhaarNumber string `json:"aadhaar_number"`
}

type setuInitResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (p *SetuAadharProvider) InitiateVerification(ctx context.Context, aadharNumber string) (string, error) {
	url := fmt.Sprintf("%s/api/okyc/requests", p.config.BaseURL)

	reqBody, _ := json.Marshal(setuInitRequest{AadhaarNumber: aadharNumber})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("setu api failed with status: %d", resp.StatusCode)
	}

	var res setuInitResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if res.Status != "otp_sent" && res.Status != "initiated" { // Check Setu specific success status
		return "", fmt.Errorf("aadhaar initiation failed: %s", res.Message)
	}

	return res.ID, nil
}

type setuVerifyRequest struct {
	OTP       string `json:"otp"`
	RequestID string `json:"request_id"`
}

type setuVerifyResponse struct {
	Status string `json:"status"`
	Data   struct {
		Name string `json:"name"`
	} `json:"data"`
}

func (p *SetuAadharProvider) VerifyOTP(ctx context.Context, transactionID string, otp string) (*AadharVerificationResult, error) {
	url := fmt.Sprintf("%s/api/okyc/requests/%s/verify", p.config.BaseURL, transactionID)

	reqBody, _ := json.Marshal(map[string]string{"otp": otp})
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("setu verify api failed with status: %d", resp.StatusCode)
	}

	var res setuVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if res.Status != "success" && res.Status != "completed" {
		return &AadharVerificationResult{Success: false}, errors.New("verification failed or pending")
	}

	return &AadharVerificationResult{
		Success:     true,
		Name:        res.Data.Name,
		ReferenceID: transactionID,
	}, nil
}

func (p *SetuAadharProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-client-id", p.config.ClientID)
	req.Header.Set("x-client-secret", p.config.ClientSecret)
	if p.config.ProductInstanceID != "" {
		req.Header.Set("x-product-instance-id", p.config.ProductInstanceID)
	}
}
