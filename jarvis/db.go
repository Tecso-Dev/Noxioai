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

func UpsertCompany(ctx context.Context, db *sql.DB, name, website, industry, country, notes string) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO companies (name, website, industry, country, raw_notes)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (website) DO UPDATE
		SET name=EXCLUDED.name, industry=EXCLUDED.industry,
		    country=EXCLUDED.country, raw_notes=EXCLUDED.raw_notes
		RETURNING id`, name, website, industry, country, notes).Scan(&id)
	return id, err
}

func UpsertLead(ctx context.Context, db *sql.DB, companyID int64, score int, tier, reasoning, problem, offer string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO leads (company_id, score, tier, reasoning, observed_problem, suggested_offer)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (company_id) DO UPDATE
		SET score=EXCLUDED.score, tier=EXCLUDED.tier, reasoning=EXCLUDED.reasoning,
		    observed_problem=EXCLUDED.observed_problem, suggested_offer=EXCLUDED.suggested_offer,
		    updated_at=now()`, companyID, score, tier, reasoning, problem, offer)
	return err
}

func AddContact(ctx context.Context, db *sql.DB, companyID int64, name, role, email, linkedin string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO contacts (company_id, name, role, email, linkedin)
		SELECT $1,$2,$3,$4,$5
		WHERE NOT EXISTS (
			SELECT 1 FROM contacts WHERE company_id=$1 AND name=$2 AND email=$4
		)`, companyID, name, role, email, linkedin)
	return err
}

func AddExperience(ctx context.Context, db *sql.DB, agent, input, decision, result, lesson string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO experiences (agent, input, decision, result, lesson)
		VALUES ($1,$2,$3,$4,$5)`, agent, input, decision, result, lesson)
	return err
}
