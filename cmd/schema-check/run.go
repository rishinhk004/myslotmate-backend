// schema-check inspects the payout_methods table schema and migration status
package main

import (
	"context"
	"fmt"
	"log"
	"myslotmate-backend/internal/config"
	"myslotmate-backend/internal/db"
	"strings"
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

	// 1. Check payout_methods table structure
	fmt.Println("\n========== PAYOUT_METHODS TABLE SCHEMA ==========")
	rows, err := sqlDB.QueryContext(queryCtx, `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_default,
			ordinal_position
		FROM information_schema.columns
		WHERE table_name = 'payout_methods'
		ORDER BY ordinal_position
	`)
	if err != nil {
		log.Fatalf("failed to query columns: %v", err)
	}
	defer rows.Close()

	fmt.Printf("%-20s %-15s %-10s %-40s\n", "Column", "Type", "Nullable", "Default")
	fmt.Println(strings.Repeat("-", 85))

	for rows.Next() {
		var columnName, dataType, isNullable string
		var columnDefault, ordinalPos interface{}

		if err := rows.Scan(&columnName, &dataType, &isNullable, &columnDefault, &ordinalPos); err != nil {
			log.Fatalf("scan error: %v", err)
		}

		def := "NULL"
		if columnDefault != nil {
			def = fmt.Sprintf("%v", columnDefault)
		}

		fmt.Printf("%-20s %-15s %-10s %-40s\n", columnName, dataType, isNullable, def)
	}

	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}

	// 2. Check foreign key constraints
	fmt.Println("\n========== FOREIGN KEY CONSTRAINTS ==========")
	fkRows, err := sqlDB.QueryContext(queryCtx, `
		SELECT
			constraint_name,
			column_name,
			referenced_table_name,
			referenced_column_name
		FROM information_schema.key_column_usage
		WHERE table_name = 'payout_methods'
		AND referenced_table_name IS NOT NULL
	`)
	if err != nil {
		log.Fatalf("failed to query FK constraints: %v", err)
	}
	defer fkRows.Close()

	fmt.Printf("%-40s %-20s %-20s %-20s\n", "Constraint", "Column", "Ref Table", "Ref Column")
	fmt.Println(strings.Repeat("-", 100))

	for fkRows.Next() {
		var constraintName, columnName, refTable, refColumn interface{}
		if err := fkRows.Scan(&constraintName, &columnName, &refTable, &refColumn); err != nil {
			log.Fatalf("scan error: %v", err)
		}
		fmt.Printf("%-40v %-20v %-20v %-20v\n", constraintName, columnName, refTable, refColumn)
	}

	if err = fkRows.Err(); err != nil {
		log.Fatal(err)
	}

	// 3. Check PostgreSQL constraints directly
	fmt.Println("\n========== POSTGRESQL CONSTRAINT DETAILS ==========")
	constRows, err := sqlDB.QueryContext(queryCtx, `
		SELECT
			constraint_name,
			constraint_type,
			is_deferrable,
			initially_deferred
		FROM information_schema.table_constraints
		WHERE table_name = 'payout_methods'
		AND constraint_type IN ('FOREIGN KEY', 'PRIMARY KEY', 'UNIQUE', 'CHECK')
	`)
	if err != nil {
		log.Fatalf("failed to query constraints: %v", err)
	}
	defer constRows.Close()

	fmt.Printf("%-40s %-15s %-12s %-18s\n", "Constraint Name", "Type", "Deferrable", "Initially Deferred")
	fmt.Println(strings.Repeat("-", 85))

	for constRows.Next() {
		var constraintName, constraintType string
		var deferrable, initiallyDeferred interface{}

		if err := constRows.Scan(&constraintName, &constraintType, &deferrable, &initiallyDeferred); err != nil {
			log.Fatalf("scan error: %v", err)
		}

		fmt.Printf("%-40s %-15s %-12v %-18v\n", constraintName, constraintType, deferrable, initiallyDeferred)
	}

	if err = constRows.Err(); err != nil {
		log.Fatal(err)
	}

	// 4. Check migration history
	fmt.Println("\n========== MIGRATION HISTORY ==========")
	migRows, err := sqlDB.QueryContext(queryCtx, `
		SELECT
			version,
			description,
			type,
			installed_on,
			success
		FROM schema_migrations
		WHERE description LIKE '%payout%'
		   OR version >= '20260324'
		ORDER BY installed_on DESC
		LIMIT 10
	`)
	if err != nil {
		// Try alternative migration tracking table
		migRows, err = sqlDB.QueryContext(queryCtx, `
			SELECT
				name,
				executed_at
			FROM schema_versions
			WHERE name LIKE '%payout%'
			   OR name >= '20260324'
			ORDER BY executed_at DESC
			LIMIT 10
		`)
		if err != nil {
			log.Println("⚠️  Could not find migration history (neither schema_migrations nor schema_versions exists)")
			log.Println("    Migrations might be tracked by a migration tool (e.g., golang-migrate, Flyway, Prisma)")
		} else {
			defer migRows.Close()

			fmt.Printf("%-50s %-30s\n", "Migration Name", "Executed At")
			fmt.Println(strings.Repeat("-", 80))

			for migRows.Next() {
				var name string
				var executedAt interface{}

				if err := migRows.Scan(&name, &executedAt); err != nil {
					log.Fatalf("scan error: %v", err)
				}

				fmt.Printf("%-50s %-30v\n", name, executedAt)
			}

			if err = migRows.Err(); err != nil {
				log.Fatal(err)
			}
		}
	} else {
		defer migRows.Close()

		fmt.Printf("%-10s %-50s %-10s %-30s %-10s\n", "Version", "Description", "Type", "Installed On", "Success")
		fmt.Println(strings.Repeat("-", 110))

		for migRows.Next() {
			var version int
			var description, migrationType string
			var installedOn interface{}
			var success interface{}

			if err := migRows.Scan(&version, &description, &migrationType, &installedOn, &success); err != nil {
				log.Fatalf("scan error: %v", err)
			}

			fmt.Printf("%-10d %-50s %-10s %-30v %-10v\n", version, description, migrationType, installedOn, success)
		}

		if err = migRows.Err(); err != nil {
			log.Fatal(err)
		}
	}

	// 5. Try to INSERT NULL into host_id and see what happens
	fmt.Println("\n========== TESTING NULL INSERT ==========")
	var testID string
	err = sqlDB.QueryRowContext(queryCtx, `
		INSERT INTO payout_methods (
			host_id,
			type,
			bank_name,
			last_four_digits,
			is_verified,
			is_primary
		) VALUES (
			NULL,
			'bank',
			'Test Bank',
			'0000',
			false,
			false
		)
		RETURNING id
	`).Scan(&testID)

	if err != nil {
		fmt.Printf("❌ INSERT with NULL host_id FAILED: %v\n", err)
		fmt.Println("   This indicates the FK constraint is still enforced for NULL values or host_id is still NOT NULL")
	} else {
		fmt.Printf("✅ INSERT with NULL host_id SUCCEEDED: %s\n", testID)
		fmt.Println("   The migration appears to have been applied successfully!")

		// Clean up test row
		_, _ = sqlDB.ExecContext(queryCtx, `DELETE FROM payout_methods WHERE id = $1`, testID)
	}

	log.Println("\n✅ Schema check complete!")
}
