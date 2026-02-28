// Package db provides a PostgreSQL connection for the backend.
// Set DATABASE_URL in env.
package db

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens a connection to PostgreSQL using connURL.
func Open(connURL string) (*sql.DB, error) {
	if connURL == "" {
		return nil, sql.ErrNoRows // or a custom error; caller should check config
	}
	return sql.Open("pgx", connURL)
}

// OpenWithContext is like Open but also pings the database with the given context.
func OpenWithContext(ctx context.Context, connURL string) (*sql.DB, error) {
	db, err := Open(connURL)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
