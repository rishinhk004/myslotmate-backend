// migrate runs SQL migrations against the database (no Docker needed).
// Run: go run ./cmd/migrate
// Requires: DATABASE_URL in .env
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"myslotmate-backend/internal/config"
	"myslotmate-backend/internal/db"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	connURL := cfg.Database.URL
	if connURL == "" {
		log.Fatal("DATABASE_URL is not set in .env")
	}

	// Fix duplicate prefix if present
	if strings.HasPrefix(connURL, "DATABASE_URL=") {
		connURL = strings.TrimPrefix(connURL, "DATABASE_URL=")
	}

	log.Printf("connecting to database...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sqlDB, err := db.OpenWithContext(ctx, connURL)
	if err != nil {
		log.Fatalf("connection failed: %v", err)
	}
	defer sqlDB.Close()

	migrationsDir := "migrations"
	if d := os.Getenv("MIGRATIONS_DIR"); d != "" {
		migrationsDir = d
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("cannot read migrations dir %q: %v", migrationsDir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		log.Printf("no migrations found in %s", migrationsDir)
		return
	}

	log.Printf("running %d migration(s)...", len(files))

	for _, name := range files {
		path := filepath.Join(migrationsDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("cannot read %s: %v", path, err)
		}

		log.Printf("  -> %s", name)

		_, err = sqlDB.ExecContext(ctx, string(content))
		if err != nil {
			log.Fatalf("migration %s failed: %v", name, err)
		}
	}

	fmt.Println("Migrations complete.")
}
