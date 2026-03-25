// check-migrations queries the migration status and table constraints
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
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env file not found, relying on environment variables")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.Database.URL == "" {
		log.Fatal("❌ DATABASE_URL is not set in .env")
	}

	// Connect
	connCtx, connCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer connCancel()

	sqlDB, err := db.OpenWithContext(connCtx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer sqlDB.Close()

	queryCtx, queryCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer queryCancel()

	// 1. Check applied migrations
	fmt.Println("\n========== APPLIED MIGRATIONS ==========")
	migRows, err := sqlDB.QueryContext(queryCtx, `
		SELECT version, applied_at 
		FROM schema_migrations 
		ORDER BY version
	`)
	if err != nil {
		log.Fatalf("failed to query migrations: %v", err)
	}
	defer migRows.Close()

	var migrations []string
	for migRows.Next() {
		var version, appliedAt string
		if err := migRows.Scan(&version, &appliedAt); err != nil {
			log.Fatalf("scan error: %v", err)
		}
		migrations = append(migrations, version)
		fmt.Printf("  ✓ %s (applied at %s)\n", version, appliedAt)
	}

	if len(migrations) == 0 {
		fmt.Println("  (No migrations applied)")
	}

	// Check specifically for the nullable migration
	fmt.Println("\n========== MIGRATION STATUS CHECK ==========")
	found := false
	for _, m := range migrations {
		if m == "20260324130000_make_payout_methods_host_id_nullable.sql" {
			found = true
			break
		}
	}
	if found {
		fmt.Println("✓ make_payout_methods_host_id_nullable migration is RECORDED as applied")
	} else {
		fmt.Println("✗ make_payout_methods_host_id_nullable migration is NOT recorded in schema_migrations")
	}

	// 2. Check host_id column constraints in payout_methods
	fmt.Println("\n========== PAYOUT_METHODS HOST_ID CONSTRAINTS ==========")

	// Check if nullable
	var isNullable string
	err = sqlDB.QueryRowContext(queryCtx, `
		SELECT is_nullable 
		FROM information_schema.columns 
		WHERE table_name = 'payout_methods' AND column_name = 'host_id'
	`).Scan(&isNullable)
	if err != nil {
		log.Fatalf("failed to check nullable: %v", err)
	}
	fmt.Printf("host_id is_nullable: %s\n", isNullable)
	if isNullable == "YES" {
		fmt.Println("  ✓ Column IS nullable (migration applied correctly)")
	} else {
		fmt.Println("  ✗ Column is NOT nullable (migration may have failed)")
	}

	// Check foreign key constraints
	fmt.Println("\nForeign Key Constraints:")
	fkRows, err := sqlDB.QueryContext(queryCtx, `
		SELECT 
			constraint_name,
			column_name,
			referenced_table_name,
			referenced_column_name
		FROM information_schema.referential_constraints
		WHERE constraint_name LIKE 'payout_methods_%'
		UNION
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name as referenced_table_name,
			ccu.column_name as referenced_column_name
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.key_column_usage AS kcu
			ON tc.constraint_name = kcu.constraint_name
		JOIN information_schema.constraint_column_usage AS ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.table_name = 'payout_methods'
		AND tc.constraint_type = 'FOREIGN KEY'
	`)
	if err != nil {
		// Try alternative PostgreSQL query
		fkRows, err = sqlDB.QueryContext(queryCtx, `
			SELECT
				constraint_name,
				column_name
			FROM information_schema.key_column_usage
			WHERE table_name = 'payout_methods'
			AND constraint_name LIKE '%host%'
		`)
		if err != nil {
			log.Fatalf("failed to query FK constraints: %v", err)
		}
	}
	defer fkRows.Close()

	hasFKResult := false
	for fkRows.Next() {
		hasFKResult = true
		var constraintName, columnName string
		var refTable, refColumn interface{}
		if err := fkRows.Scan(&constraintName, &columnName, &refTable, &refColumn); err != nil {
			// Try simpler scan
			if err := fkRows.Scan(&constraintName, &columnName); err != nil {
				log.Printf("scan error: %v", err)
				continue
			}
			fmt.Printf("  - %s on column %s\n", constraintName, columnName)
		} else {
			fmt.Printf("  - %s on column %s → %s(%s)\n", constraintName, columnName, refTable, refColumn)
		}
	}

	if !hasFKResult {
		fmt.Println("  (No FK constraints found in query result)")
	}

	// 3. Show actual table structure
	fmt.Println("\n========== PAYOUT_METHODS TABLE STRUCTURE ==========")
	structRows, err := sqlDB.QueryContext(queryCtx, `
		\d payout_methods
	`)
	if err != nil {
		// PostgreSQL doesn't support \d in SQL; use a different approach
		structRows.Close()
		
		// Get column info via information_schema
		colRows, err := sqlDB.QueryContext(queryCtx, `
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_name = 'payout_methods'
			ORDER BY ordinal_position
		`)
		if err != nil {
			log.Fatalf("failed to get columns: %v", err)
		}
		defer colRows.Close()

		fmt.Printf("%-25s %-20s %s\n", "Column", "Type", "Nullable")
		fmt.Println(string(make([]byte, 50)))
		for colRows.Next() {
			var colName, colType, nullable string
			if err := colRows.Scan(&colName, &colType, &nullable); err != nil {
				log.Fatalf("scan error: %v", err)
			}
			marker := "NO"
			if nullable == "YES" {
				marker = "YES ✓"
			}
			fmt.Printf("%-25s %-20s %s\n", colName, colType, marker)
		}
	}

	fmt.Println("\n========== SUMMARY ==========")
	fmt.Printf("Nullable: %s\n", isNullable)
	if isNullable == "YES" && found {
		fmt.Println("✓ Database state matches migration expectations")
	} else {
		fmt.Println("✗ Database state may not match migrations")
	}
}
