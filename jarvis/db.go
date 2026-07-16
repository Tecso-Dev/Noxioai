package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
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

// InitSchema applies schema.sql (idempotent — every statement is IF NOT EXISTS)
// then ensures the default owner exists and backfills any ownerless CRM rows
// to that owner (PRODUCT-BUILD.md Phase P1). Returns the owner's user id.
func InitSchema(ctx context.Context, db *sql.DB) (int64, error) {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	ownerID, err := defaultOwnerID(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("default owner: %w", err)
	}
	if err := backfillOwner(ctx, tx, ownerID); err != nil {
		return 0, err
	}
	if err := requireOwners(ctx, tx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return ownerID, nil
}

type queryExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// defaultOwnerID resolves the platform owner used by CLI callers and the P1
// backfill: JARVIS_OWNER_EMAIL (default sobhan@noxioai.com), creating the
// user with a random password if it doesn't exist yet. Idempotent.
func defaultOwnerID(ctx context.Context, db queryExecer) (int64, error) {
	email := envOr("JARVIS_OWNER_EMAIL", "sobhan@noxioai.com")
	var id int64
	err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return 0, err
	}
	hash, err := hashPassword(hex.EncodeToString(raw))
	if err != nil {
		return 0, err
	}
	err = db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, name, is_admin) VALUES ($1,$2,'Sobhan',true)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`, email, hash).Scan(&id)
	return id, err
}

// backfillOwner assigns ownerID to every existing CRM row that has none yet
// (PRODUCT-BUILD.md Phase P1: Sobhan's pre-multi-tenant data stays his).
func backfillOwner(ctx context.Context, db queryExecer, ownerID int64) error {
	for _, table := range []string{"companies", "contacts", "leads", "outreach", "experiences"} {
		if _, err := db.ExecContext(ctx, `UPDATE `+table+` SET owner_id=$1 WHERE owner_id IS NULL`, ownerID); err != nil {
			return fmt.Errorf("backfill %s: %w", table, err)
		}
	}
	return nil
}

// requireOwners closes the legacy-migration window after backfill. From this
// point forward PostgreSQL itself rejects ownerless CRM rows, including writes
// that bypass the Go helpers.
func requireOwners(ctx context.Context, db queryExecer) error {
	for _, table := range []string{"companies", "contacts", "leads", "outreach", "experiences"} {
		if _, err := db.ExecContext(ctx, `ALTER TABLE `+table+` ALTER COLUMN owner_id SET NOT NULL`); err != nil {
			return fmt.Errorf("require %s.owner_id: %w", table, err)
		}
	}
	return nil
}

func UpsertCompany(ctx context.Context, db *sql.DB, ownerID int64, name, website, industry, country, notes string) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO companies (owner_id, name, website, industry, country, raw_notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (owner_id, website) DO UPDATE
		SET name=EXCLUDED.name, industry=EXCLUDED.industry,
		    country=EXCLUDED.country, raw_notes=EXCLUDED.raw_notes
		RETURNING id`, ownerID, name, website, industry, country, notes).Scan(&id)
	return id, err
}

func UpsertLead(ctx context.Context, db *sql.DB, ownerID, companyID int64, score int, tier, reasoning, problem, offer string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO leads (owner_id, company_id, score, tier, reasoning, observed_problem, suggested_offer)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (owner_id, company_id) DO UPDATE
		SET score=EXCLUDED.score, tier=EXCLUDED.tier, reasoning=EXCLUDED.reasoning,
		    observed_problem=EXCLUDED.observed_problem, suggested_offer=EXCLUDED.suggested_offer,
		    updated_at=now()`, ownerID, companyID, score, tier, reasoning, problem, offer)
	return err
}

func AddContact(ctx context.Context, db *sql.DB, ownerID, companyID int64, name, role, email, linkedin string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO contacts (owner_id, company_id, name, role, email, linkedin)
		SELECT $1,$2,$3,$4,$5,$6
		WHERE NOT EXISTS (
			SELECT 1 FROM contacts WHERE owner_id=$1 AND company_id=$2 AND name=$3 AND email=$5
		)`, ownerID, companyID, name, role, email, linkedin)
	return err
}

func AddExperience(ctx context.Context, db *sql.DB, ownerID int64, agent, input, decision, result, lesson string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO experiences (owner_id, agent, input, decision, result, lesson)
		VALUES ($1,$2,$3,$4,$5,$6)`, ownerID, agent, input, decision, result, lesson)
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

func GetLead(ctx context.Context, db *sql.DB, ownerID, id int64) (*leadRow, error) {
	var l leadRow
	err := db.QueryRowContext(ctx, `
		SELECT l.id, l.company_id, c.name, COALESCE(c.website,''), COALESCE(c.industry,''),
		       COALESCE(c.raw_notes,''), COALESCE(l.score,0), COALESCE(l.tier,''),
		       l.reasoning, COALESCE(l.observed_problem,''), COALESCE(l.suggested_offer,'')
		FROM leads l JOIN companies c ON c.id = l.company_id
		WHERE l.id = $1 AND l.owner_id = $2`, id, ownerID).
		Scan(&l.ID, &l.CompanyID, &l.Name, &l.Website, &l.Industry, &l.Notes,
			&l.Score, &l.Tier, &l.Reasoning, &l.ObservedProblem, &l.SuggestedOffer)
	return &l, err
}

type leadListRow struct {
	ID      int64
	Score   int
	Tier    string
	Status  string
	Name    string
	Website string
}

// ListLeads returns this owner's leads, highest score first. Never returns
// another owner's rows — used by both PrintLeads and the tenancy isolation test.
func ListLeads(ctx context.Context, db *sql.DB, ownerID int64) ([]leadListRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.id, COALESCE(l.score,0), COALESCE(l.tier,''), l.status, c.name, COALESCE(c.website,'')
		FROM leads l JOIN companies c ON c.id = l.company_id
		WHERE l.owner_id = $1
		ORDER BY l.score DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []leadListRow
	for rows.Next() {
		var l leadListRow
		if err := rows.Scan(&l.ID, &l.Score, &l.Tier, &l.Status, &l.Name, &l.Website); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func PrintLeads(ctx context.Context, db *sql.DB, ownerID int64) error {
	leads, err := ListLeads(ctx, db, ownerID)
	if err != nil {
		return err
	}
	fmt.Printf("%4s  %5s  %-6s  %-9s  %-30s  %s\n", "ID", "SCORE", "TIER", "STATUS", "COMPANY", "WEBSITE")
	for _, l := range leads {
		fmt.Printf("%4d  %5d  %-6s  %-9s  %-30s  %s\n", l.ID, l.Score, l.Tier, l.Status, oneLine(l.Name, 30), l.Website)
	}
	return nil
}

func CreateOutreach(ctx context.Context, db *sql.DB, ownerID, leadID int64, channel, draft string) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO outreach (owner_id, lead_id, channel, draft)
		SELECT $1,$2,$3,$4 WHERE EXISTS (SELECT 1 FROM leads WHERE id=$2 AND owner_id=$1)
		RETURNING id`,
		ownerID, leadID, channel, draft).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("lead %d: not found for this owner", leadID)
	}
	return id, err
}

type outreachRow struct {
	ID      int64
	LeadID  int64
	Channel string
	Draft   string
}

// ListOutreach returns this owner's outreach rows, optionally filtered to one
// lead (leadID == 0 lists all). Never returns another owner's rows.
func ListOutreach(ctx context.Context, db *sql.DB, ownerID, leadID int64) ([]outreachRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, lead_id, channel, draft FROM outreach
		WHERE owner_id = $1 AND ($2 = 0 OR lead_id = $2)
		ORDER BY id`, ownerID, leadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []outreachRow
	for rows.Next() {
		var o outreachRow
		if err := rows.Scan(&o.ID, &o.LeadID, &o.Channel, &o.Draft); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

type contactRow struct {
	ID       int64
	Name     string
	Email    string
	Linkedin string
}

// ListContacts returns this owner's contacts for a company. Never returns
// another owner's rows.
func ListContacts(ctx context.Context, db *sql.DB, ownerID, companyID int64) ([]contactRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(name,''), COALESCE(email,''), COALESCE(linkedin,'') FROM contacts
		WHERE owner_id = $1 AND company_id = $2
		ORDER BY id`, ownerID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []contactRow
	for rows.Next() {
		var c contactRow
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Linkedin); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ApproveOutreach flips the human gate (Principle 1) and returns the draft to send.
func ApproveOutreach(ctx context.Context, db *sql.DB, ownerID, id int64) (string, error) {
	var draft string
	err := db.QueryRowContext(ctx, `
		UPDATE outreach SET approved = TRUE WHERE id = $1 AND owner_id = $2 RETURNING draft`, id, ownerID).Scan(&draft)
	return draft, err
}

// SetOutcome records what happened after sending and advances the lead status.
func SetOutcome(ctx context.Context, db *sql.DB, ownerID, id int64, outcome string) error {
	var leadID int64
	if err := db.QueryRowContext(ctx, `
		UPDATE outreach SET outcome = $2, sent_at = COALESCE(sent_at, now())
		WHERE id = $1 AND owner_id = $3 RETURNING lead_id`, id, outcome, ownerID).Scan(&leadID); err != nil {
		return err
	}
	status := map[string]string{"replied": "replied", "meeting": "replied", "won": "won", "lost": "lost"}[outcome]
	if status == "" {
		status = "contacted"
	}
	_, err := db.ExecContext(ctx, `UPDATE leads SET status = $2, updated_at = now() WHERE id = $1 AND owner_id = $3`, leadID, status, ownerID)
	return err
}

func RecentLessons(ctx context.Context, db *sql.DB, ownerID int64, agent string, n int) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT lesson FROM experiences
		WHERE owner_id = $1 AND agent = $2 AND lesson <> '' ORDER BY created_at DESC LIMIT $3`, ownerID, agent, n)
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
