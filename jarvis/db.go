package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"

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

type leadRow struct {
	ID              int64
	CompanyID       int64
	Name            string
	Website         string
	Industry        string
	Notes           string
	Score           int
	Tier            string
	Reasoning       string
	ObservedProblem string
	SuggestedOffer  string
}

func GetLead(ctx context.Context, db *sql.DB, id int64) (*leadRow, error) {
	var l leadRow
	err := db.QueryRowContext(ctx, `
		SELECT l.id, l.company_id, c.name, COALESCE(c.website,''), COALESCE(c.industry,''),
		       COALESCE(c.raw_notes,''), COALESCE(l.score,0), COALESCE(l.tier,''),
		       l.reasoning, COALESCE(l.observed_problem,''), COALESCE(l.suggested_offer,'')
		FROM leads l JOIN companies c ON c.id = l.company_id
		WHERE l.id = $1`, id).
		Scan(&l.ID, &l.CompanyID, &l.Name, &l.Website, &l.Industry, &l.Notes,
			&l.Score, &l.Tier, &l.Reasoning, &l.ObservedProblem, &l.SuggestedOffer)
	return &l, err
}

func PrintLeads(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `
		SELECT l.id, COALESCE(l.score,0), COALESCE(l.tier,''), l.status, c.name, COALESCE(c.website,'')
		FROM leads l JOIN companies c ON c.id = l.company_id
		ORDER BY l.score DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	fmt.Printf("%4s  %5s  %-6s  %-9s  %-30s  %s\n", "ID", "SCORE", "TIER", "STATUS", "COMPANY", "WEBSITE")
	for rows.Next() {
		var id int64
		var score int
		var tier, status, name, website string
		if err := rows.Scan(&id, &score, &tier, &status, &name, &website); err != nil {
			return err
		}
		fmt.Printf("%4d  %5d  %-6s  %-9s  %-30s  %s\n", id, score, tier, status, oneLine(name, 30), website)
	}
	return rows.Err()
}

func CreateOutreach(ctx context.Context, db *sql.DB, leadID int64, channel, draft string) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO outreach (lead_id, channel, draft) VALUES ($1,$2,$3) RETURNING id`,
		leadID, channel, draft).Scan(&id)
	return id, err
}

// ApproveOutreach flips the human gate (Principle 1) and returns the draft to send.
func ApproveOutreach(ctx context.Context, db *sql.DB, id int64) (string, error) {
	var draft string
	err := db.QueryRowContext(ctx, `
		UPDATE outreach SET approved = TRUE WHERE id = $1 RETURNING draft`, id).Scan(&draft)
	return draft, err
}

// SetOutcome records what happened after sending and advances the lead status.
func SetOutcome(ctx context.Context, db *sql.DB, id int64, outcome string) error {
	var leadID int64
	if err := db.QueryRowContext(ctx, `
		UPDATE outreach SET outcome = $2, sent_at = COALESCE(sent_at, now())
		WHERE id = $1 RETURNING lead_id`, id, outcome).Scan(&leadID); err != nil {
		return err
	}
	status := map[string]string{"replied": "replied", "meeting": "replied", "won": "won", "lost": "lost"}[outcome]
	if status == "" {
		status = "contacted"
	}
	_, err := db.ExecContext(ctx, `UPDATE leads SET status = $2, updated_at = now() WHERE id = $1`, leadID, status)
	return err
}

func RecentLessons(ctx context.Context, db *sql.DB, agent string, n int) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT lesson FROM experiences
		WHERE agent = $1 AND lesson <> '' ORDER BY created_at DESC LIMIT $2`, agent, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func mustDB() *sql.DB {
	db, err := OpenDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ cannot reach Postgres (docker compose up -d):", err)
		os.Exit(1)
	}
	return db
}
