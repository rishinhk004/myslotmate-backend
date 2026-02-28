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

type Config struct {
	HTTPPort string
	Firebase FirebaseConfig
	Database DatabaseConfig
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
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

