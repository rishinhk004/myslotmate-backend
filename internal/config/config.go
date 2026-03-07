package config

import (
	"os"

	"github.com/joho/godotenv"
)

type FirebaseConfig struct {
	CredentialsFile string
	ProjectID       string
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

// RazorpayConfig holds RazorpayX Payouts API credentials.
type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	AccountNumber string
	WebhookSecret string
}

type Config struct {
	HTTPPort string
	Firebase FirebaseConfig
	Database DatabaseConfig
	Setu     SetuConfig
	Razorpay RazorpayConfig
}

// Load reads configuration from environment variables (optionally via .env).
func Load() (*Config, error) {
	// Load .env if present; ignore errors because it's optional.
	_ = godotenv.Load()

	cfg := &Config{
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		Firebase: FirebaseConfig{
			CredentialsFile: getEnv("FIREBASE_CREDENTIALS_FILE", "config/firebase-service-account.json"),
			ProjectID:       getEnv("FIREBASE_PROJECT_ID", "myslotmate-25994"),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", ""),
		},
		Setu: SetuConfig{
			BaseURL:           getEnv("SETU_BASE_URL", "https://uat.setu.co"),
			ClientID:          getEnv("SETU_CLIENT_ID", ""),
			ClientSecret:      getEnv("SETU_CLIENT_SECRET", ""),
			ProductInstanceID: getEnv("SETU_PRODUCT_INSTANCE_ID", ""),
		},
		Razorpay: RazorpayConfig{
			KeyID:         getEnv("RAZORPAY_KEY_ID", ""),
			KeySecret:     getEnv("RAZORPAY_KEY_SECRET", ""),
			AccountNumber: getEnv("RAZORPAY_ACCOUNT_NUMBER", ""),
			WebhookSecret: getEnv("RAZORPAY_WEBHOOK_SECRET", ""),
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
