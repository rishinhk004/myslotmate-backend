// checkdb connects to the database and runs a simple query to verify it works.
package main

import (
	"context"
	"fmt"
	"log"
	"myslotmate-backend/internal/config"
	"myslotmate-backend/internal/db"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// 1️⃣ Load .env FIRST
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found, relying on environment variables")
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

	connCtx, connCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer connCancel()

	sqlDB, err := db.OpenWithContext(connCtx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer sqlDB.Close()

	queryCtx, queryCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer queryCancel()

	var one int
	err = sqlDB.QueryRowContext(queryCtx, "SELECT 1").Scan(&one)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}

	var dbName string
	err = sqlDB.QueryRowContext(queryCtx, "SELECT current_database()").Scan(&dbName)
	if err != nil {
		log.Printf("could not get database name: %v", err)
		dbName = "unknown"
	}

	fmt.Println("🎉 Database connection OK")
	fmt.Printf("Database: %s\n", dbName)
	fmt.Printf("Ping: SELECT 1 = %d\n", one)
}
