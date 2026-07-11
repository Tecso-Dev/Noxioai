package main

import (
	"context"
	"database/sql"
	_ "embed"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed schema.sql
var schemaSQL string

// OpenDB connects to the JARVIS Postgres (SPEC §3: docker-compose, port 5434).
func OpenDB() (*sql.DB, error) {
	dsn := envOr("JARVIS_DB_URL", "postgres://jarvis:jarvis@localhost:5434/jarvis?sslmode=disable")
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

// InitSchema applies schema.sql (idempotent — every statement is IF NOT EXISTS).
func InitSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaSQL)
	return err
}
