package config

import (
	"os"

	"github.com/joho/godotenv"
)

type FirebaseConfig struct {
	CredentialsFile string
	ProjectID       string
}

// S3Config holds AWS S3 settings for file uploads.
type S3Config struct {
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	URL string // DATABASE_URL, Postgres connection string
}

type SetuConfig struct {
	BaseURL           string
	ClientID          string
	ClientSecret      string
	ProductInstanceID string
}

// AadharConfig controls which KYC strategy implementation to use.
type AadharConfig struct {
	Provider string // setu | mock
	MockOTP  string
	MockName string
}

// RazorpayConfig holds Razorpay Standard payment collection credentials.
type RazorpayConfig struct {
	KeyID                string
	KeySecret            string
	WebhookSecret        string // legacy/shared webhook secret fallback
	PaymentWebhookSecret string // payment collection webhook secret
}

// CashfreeConfig holds Cashfree payout credentials.
type CashfreeConfig struct {
	BaseURL       string
	ClientID      string
	ClientSecret  string
	WebhookSecret string
	APIVersion    string
}

type Config struct {
	HTTPPort          string
	AdminEmail        string
	RenderExternalURL string // RENDER_EXTERNAL_URL, used for self-ping keep-alive
	Firebase          FirebaseConfig
	Database          DatabaseConfig
	Aadhar            AadharConfig
	Setu              SetuConfig
	Razorpay          RazorpayConfig
	Cashfree          CashfreeConfig
	S3                S3Config
}

// Load reads configuration from environment variables (optionally via .env).
func Load() (*Config, error) {
	// Load .env if present; ignore errors because it's optional.
	_ = godotenv.Load()

	cfg := &Config{
		HTTPPort:          getEnv("HTTP_PORT", "8080"),
		AdminEmail:        getEnv("ADMIN_EMAIL", ""),
		RenderExternalURL: getEnv("RENDER_EXTERNAL_URL", ""),
		Firebase: FirebaseConfig{
			CredentialsFile: getEnv("FIREBASE_CREDENTIALS_FILE", "config/firebase-service-account.json"),
			ProjectID:       getEnv("FIREBASE_PROJECT_ID", "myslotmate-25994"),
		},
		S3: S3Config{
			Bucket:    getEnv("AWS_S3_BUCKET", ""),
			Region:    getEnv("AWS_S3_REGION", "ap-south-1"),
			AccessKey: getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", ""),
		},
		Aadhar: AadharConfig{
			Provider: getEnv("AADHAR_PROVIDER", "setu"),
			MockOTP:  getEnv("AADHAR_MOCK_OTP", "123456"),
			MockName: getEnv("AADHAR_MOCK_NAME", "Test User"),
		},
		Setu: SetuConfig{
			BaseURL:           getEnv("SETU_BASE_URL", "https://uat.setu.co"),
			ClientID:          getEnv("SETU_CLIENT_ID", ""),
			ClientSecret:      getEnv("SETU_CLIENT_SECRET", ""),
			ProductInstanceID: getEnv("SETU_PRODUCT_INSTANCE_ID", ""),
		},
		Razorpay: RazorpayConfig{
			KeyID:                getEnv("RAZORPAY_KEY_ID", ""),
			KeySecret:            getEnv("RAZORPAY_KEY_SECRET", ""),
			WebhookSecret:        getEnv("RAZORPAY_WEBHOOK_SECRET", ""),
			PaymentWebhookSecret: getEnv("RAZORPAY_PAYMENT_WEBHOOK_SECRET", ""),
		},
		Cashfree: CashfreeConfig{
			BaseURL:       getEnv("CASHFREE_BASE_URL", "https://payout-api.cashfree.com"),
			ClientID:      getEnv("CASHFREE_CLIENT_ID", ""),
			ClientSecret:  getEnv("CASHFREE_CLIENT_SECRET", ""),
			WebhookSecret: getEnv("CASHFREE_WEBHOOK_SECRET", ""),
			APIVersion:    getEnv("CASHFREE_API_VERSION", "2024-01-01"),
		},
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
