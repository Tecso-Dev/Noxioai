package main

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// TestTenantIsolation is the P1 safety net (PRODUCT-BUILD.md): two owners
// each get a company+lead+contact+outreach through the REAL CRM functions,
// then every list/get path is asserted to return ONLY the caller's own rows.
// A missing `owner_id` filter anywhere in the CRM read path makes this fail.
func TestTenantIsolation(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Skipf("postgres not reachable (start with `docker compose up -d`): %v", err)
	}
	// Registered first so it runs LAST (t.Cleanup is LIFO) — the data cleanup
	// below needs a live connection, unlike a plain `defer` in this function
	// which would close db before t.Cleanup callbacks ever run.
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	if _, err := InitSchema(ctx, db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	nonce := time.Now().UnixNano()
	ownerA := mustTestUser(t, ctx, db, fmt.Sprintf("tenant-a-%d@test.invalid", nonce))
	ownerB := mustTestUser(t, ctx, db, fmt.Sprintf("tenant-b-%d@test.invalid", nonce))

	// Both owners target the SAME website — proves the per-owner unique
	// (owner_id, website) replaced the old global UNIQUE(website).
	website := fmt.Sprintf("https://shared-target-%d.test.invalid", nonce)

	companyA, err := UpsertCompany(ctx, db, ownerA, "Acme A", website, "software", "PL", "notes a")
	if err != nil {
		t.Fatalf("UpsertCompany(A): %v (per-owner unique should allow this)", err)
	}
	companyB, err := UpsertCompany(ctx, db, ownerB, "Acme B", website, "software", "PL", "notes b")
	if err != nil {
		t.Fatalf("UpsertCompany(B): %v (per-owner unique should allow the same website for a different owner)", err)
	}
	if companyA == companyB {
		t.Fatalf("owner A and owner B collided onto the same company row (%d) — unique constraint is not owner-scoped", companyA)
	}

	if err := UpsertLead(ctx, db, ownerA, companyA, 80, "HIGH", "reasoning a", "problem a", "offer a"); err != nil {
		t.Fatalf("UpsertLead(A): %v", err)
	}
	if err := UpsertLead(ctx, db, ownerB, companyB, 90, "VIP", "reasoning b", "problem b", "offer b"); err != nil {
		t.Fatalf("UpsertLead(B): %v", err)
	}

	if err := AddContact(ctx, db, ownerA, companyA, "Alice", "CEO", "alice@a.test.invalid", ""); err != nil {
		t.Fatalf("AddContact(A): %v", err)
	}
	if err := AddContact(ctx, db, ownerB, companyB, "Bob", "CEO", "bob@b.test.invalid", ""); err != nil {
		t.Fatalf("AddContact(B): %v", err)
	}

	leadA, err := ListLeads(ctx, db, ownerA)
	if err != nil || len(leadA) != 1 {
		t.Fatalf("ListLeads(A) setup: got %d leads, err=%v", len(leadA), err)
	}
	leadB, err := ListLeads(ctx, db, ownerB)
	if err != nil || len(leadB) != 1 {
		t.Fatalf("ListLeads(B) setup: got %d leads, err=%v", len(leadB), err)
	}
	leadAID, leadBID := leadA[0].ID, leadB[0].ID

	outreachAID, err := CreateOutreach(ctx, db, ownerA, leadAID, "email", "Subject: hi A\n\nbody a")
	if err != nil {
		t.Fatalf("CreateOutreach(A): %v", err)
	}
	outreachBID, err := CreateOutreach(ctx, db, ownerB, leadBID, "email", "Subject: hi B\n\nbody b")
	if err != nil {
		t.Fatalf("CreateOutreach(B): %v", err)
	}

	t.Cleanup(func() {
		for _, tbl := range []string{"outreach", "contacts", "leads", "companies"} {
			db.ExecContext(ctx, "DELETE FROM "+tbl+" WHERE owner_id IN ($1,$2)", ownerA, ownerB)
		}
		db.ExecContext(ctx, "DELETE FROM sessions WHERE user_id IN ($1,$2)", ownerA, ownerB)
		db.ExecContext(ctx, "DELETE FROM users WHERE id IN ($1,$2)", ownerA, ownerB)
	})

	// --- leads: A's list/get must never surface B's rows, and vice versa ---
	leadsAsA, err := ListLeads(ctx, db, ownerA)
	if err != nil {
		t.Fatalf("ListLeads(A): %v", err)
	}
	assertOnlyIDs(t, "ListLeads(A)", ids(leadsAsA, func(l leadListRow) int64 { return l.ID }), leadAID)

	leadsAsB, err := ListLeads(ctx, db, ownerB)
	if err != nil {
		t.Fatalf("ListLeads(B): %v", err)
	}
	assertOnlyIDs(t, "ListLeads(B)", ids(leadsAsB, func(l leadListRow) int64 { return l.ID }), leadBID)

	if _, err := GetLead(ctx, db, ownerA, leadBID); err != sql.ErrNoRows {
		t.Fatalf("GetLead(ownerA, leadB.ID) leaked B's lead: err=%v (want sql.ErrNoRows)", err)
	}
	if _, err := GetLead(ctx, db, ownerB, leadAID); err != sql.ErrNoRows {
		t.Fatalf("GetLead(ownerB, leadA.ID) leaked A's lead: err=%v (want sql.ErrNoRows)", err)
	}

	// --- outreach: same shape ---
	outreachAsA, err := ListOutreach(ctx, db, ownerA, 0)
	if err != nil {
		t.Fatalf("ListOutreach(A): %v", err)
	}
	assertOnlyIDs(t, "ListOutreach(A)", ids(outreachAsA, func(o outreachRow) int64 { return o.ID }), outreachAID)

	outreachAsB, err := ListOutreach(ctx, db, ownerB, 0)
	if err != nil {
		t.Fatalf("ListOutreach(B): %v", err)
	}
	assertOnlyIDs(t, "ListOutreach(B)", ids(outreachAsB, func(o outreachRow) int64 { return o.ID }), outreachBID)

	// ApproveOutreach must refuse to touch another owner's row.
	if _, err := ApproveOutreach(ctx, db, ownerA, outreachBID); err != sql.ErrNoRows {
		t.Fatalf("ApproveOutreach(ownerA, outreachB.ID) leaked/mutated B's outreach: err=%v (want sql.ErrNoRows)", err)
	}

	// --- contacts: same shape ---
	contactsAsA, err := ListContacts(ctx, db, ownerA, companyA)
	if err != nil {
		t.Fatalf("ListContacts(A, companyA): %v", err)
	}
	if len(contactsAsA) != 1 || contactsAsA[0].Email != "alice@a.test.invalid" {
		t.Fatalf("ListContacts(A, companyA) = %+v, want exactly Alice", contactsAsA)
	}
	// Owner A querying B's company_id must come back empty, not B's contact.
	leakedContacts, err := ListContacts(ctx, db, ownerA, companyB)
	if err != nil {
		t.Fatalf("ListContacts(A, companyB): %v", err)
	}
	if len(leakedContacts) != 0 {
		t.Fatalf("ListContacts(ownerA, companyB) leaked %d of B's contacts: %+v", len(leakedContacts), leakedContacts)
	}

	contactsAsB, err := ListContacts(ctx, db, ownerB, companyB)
	if err != nil {
		t.Fatalf("ListContacts(B, companyB): %v", err)
	}
	if len(contactsAsB) != 1 || contactsAsB[0].Email != "bob@b.test.invalid" {
		t.Fatalf("ListContacts(B, companyB) = %+v, want exactly Bob", contactsAsB)
	}
}

func mustTestUser(t *testing.T, ctx context.Context, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, name) VALUES ($1, 'unused-test-hash', 'tenancy-test')
		RETURNING id`, email).Scan(&id)
	if err != nil {
		t.Fatalf("create test user %s: %v", email, err)
	}
	return id
}

func ids[T any](rows []T, f func(T) int64) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = f(r)
	}
	return out
}

func assertOnlyIDs(t *testing.T, label string, got []int64, want int64) {
	t.Helper()
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s = %v, want exactly [%d] — cross-tenant leak or missing row", label, got, want)
	}
}
