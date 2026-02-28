// checkdb connects to the database and runs a simple query to verify it works.
// Run: go run ./cmd/checkdb
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"

	"myslotmate-backend/internal/config"
	"myslotmate-backend/internal/db"
)

func main() {
	// 1️⃣ Load .env FIRST
	err := godotenv.Load()
	if err != nil {
		log.Fatal("❌ .env file not found in project root")
	}

	// 2️⃣ Load config (now env vars exist)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.Database.URL == "" {
		log.Fatal("❌ DATABASE_URL is not set in .env")
	}

	// Mask password in log
	url := cfg.Database.URL
	if len(url) > 40 {
		url = url[:30] + "..." + url[len(url)-10:]
	}
	log.Printf("connecting to database (%s)...", url)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlDB, err := db.OpenWithContext(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer sqlDB.Close()

	var one int
	err = sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}

	var dbName string
	err = sqlDB.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName)
	if err != nil {
		log.Printf("could not get database name: %v", err)
		dbName = "unknown"
	}

	fmt.Println("🎉 Database connection OK")
	fmt.Printf("Database: %s\n", dbName)
	fmt.Printf("Ping: SELECT 1 = %d\n", one)
}
