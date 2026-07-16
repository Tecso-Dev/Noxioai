package main

import (
	"context"
	"testing"
)

// Round-trip against the real Postgres (SPEC Phase 1 Definition of Done).
// Skips when the database is not running so `go test` stays green without docker.
func TestDBRoundTrip(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Skipf("postgres not reachable (start with `docker compose up -d`): %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	ownerID, err := InitSchema(ctx, db)
	if err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	var id int64
	err = db.QueryRowContext(ctx,
		`INSERT INTO companies (owner_id, name, website) VALUES ($1, '__test co', 'https://roundtrip.test.invalid')
		 ON CONFLICT (owner_id, website) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, ownerID).Scan(&id)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	defer db.ExecContext(ctx, `DELETE FROM companies WHERE id = $1`, id)

	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM companies WHERE id = $1`, id).Scan(&name); err != nil {
		t.Fatalf("select: %v", err)
	}
	if name != "__test co" {
		t.Fatalf("round trip: got %q, want %q", name, "__test co")
	}
}
