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

	// Ensure tracking table exists
	_, err = sqlDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	if err != nil {
		log.Fatalf("cannot create schema_migrations table: %v", err)
	}

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

	// Load already-applied migrations
	applied := make(map[string]bool)
	rows, err := sqlDB.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		log.Fatalf("cannot query schema_migrations: %v", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			log.Fatalf("scan error: %v", err)
		}
		applied[v] = true
	}
	rows.Close()

	// Filter to pending migrations
	var pending []string
	for _, f := range files {
		if !applied[f] {
			pending = append(pending, f)
		}
	}

	if len(pending) == 0 {
		fmt.Println("No pending migrations.")
		return
	}

	log.Printf("running %d migration(s)...", len(pending))

	for _, name := range pending {
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

		// Record as applied
		_, err = sqlDB.ExecContext(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING", name)
		if err != nil {
			log.Fatalf("cannot record migration %s: %v", name, err)
		}
	}

	fmt.Println("Migrations complete.")
}
