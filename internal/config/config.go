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

// RazorpayConfig holds RazorpayX Payouts API credentials.
type RazorpayConfig struct {
	KeyID         string
	KeySecret     string
	AccountNumber string
	WebhookSecret string
}

type Config struct {
	HTTPPort   string
	AdminEmail string
	Firebase   FirebaseConfig
	Database   DatabaseConfig
	Setu       SetuConfig
	Razorpay   RazorpayConfig
	S3         S3Config
}

// Load reads configuration from environment variables (optionally via .env).
func Load() (*Config, error) {
	// Load .env if present; ignore errors because it's optional.
	_ = godotenv.Load()

	cfg := &Config{
		HTTPPort:   getEnv("HTTP_PORT", "8080"),
		AdminEmail: getEnv("ADMIN_EMAIL", ""),
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
