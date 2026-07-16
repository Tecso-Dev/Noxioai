package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var crmSQL = regexp.MustCompile(`(?i)\b(?:from|join|into|update|delete\s+from)\s+(?:companies|contacts|leads|outreach|experiences)\b`)

func TestTenantRelationshipsRejectCrossOwnerRows(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Skipf("postgres not reachable (start with `docker compose up -d`): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	if _, err := InitSchema(ctx, db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	nonce := time.Now().UnixNano()
	ownerA := mustTestUser(t, ctx, db, "relationship-a-"+strconv.FormatInt(nonce, 10)+"@test.invalid")
	ownerB := mustTestUser(t, ctx, db, "relationship-b-"+strconv.FormatInt(nonce, 10)+"@test.invalid")
	var ownerlessCompanyID int64
	t.Cleanup(func() {
		for _, table := range []string{"outreach", "contacts", "leads", "companies"} {
			db.ExecContext(ctx, "DELETE FROM "+table+" WHERE owner_id IN ($1,$2)", ownerA, ownerB)
		}
		if ownerlessCompanyID != 0 {
			db.ExecContext(ctx, `DELETE FROM companies WHERE id=$1`, ownerlessCompanyID)
		}
		db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, ownerA, ownerB)
	})

	companyB, err := UpsertCompany(ctx, db, ownerB, "Tenant B", "https://relationship-"+strconv.FormatInt(nonce, 10)+".test.invalid", "software", "PL", "")
	if err != nil {
		t.Fatalf("UpsertCompany(B): %v", err)
	}
	if err := UpsertLead(ctx, db, ownerA, companyB, 70, "HIGH", "cross-owner", "problem", "offer"); err == nil {
		t.Fatal("UpsertLead accepted owner A with owner B's company")
	}
	if err := AddContact(ctx, db, ownerA, companyB, "Mallory", "CEO", "mallory@test.invalid", ""); err == nil {
		t.Fatal("AddContact accepted owner A with owner B's company")
	}

	if err := UpsertLead(ctx, db, ownerB, companyB, 90, "VIP", "owner B", "problem", "offer"); err != nil {
		t.Fatalf("UpsertLead(B): %v", err)
	}
	leadsB, err := ListLeads(ctx, db, ownerB)
	if err != nil || len(leadsB) != 1 {
		t.Fatalf("ListLeads(B): got %d, err=%v", len(leadsB), err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO outreach (owner_id, lead_id, channel, draft) VALUES ($1,$2,'email','cross-owner')`, ownerA, leadsB[0].ID); err == nil {
		t.Fatal("database accepted owner A outreach linked to owner B's lead")
	}

	err = db.QueryRowContext(ctx, `INSERT INTO companies (name, website) VALUES ('ownerless', $1) RETURNING id`, "https://ownerless-"+strconv.FormatInt(nonce, 10)+".test.invalid").Scan(&ownerlessCompanyID)
	if err == nil {
		t.Fatal("database accepted a company without owner_id")
	}
}

func TestOwnerFromSessionRejectsAnonymousRequests(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Skipf("postgres not reachable (start with `docker compose up -d`): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := InitSchema(context.Background(), db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/status", nil)
	ownerID, err := ownerFromSession(req.Context(), db, req)
	if err == nil || ownerID != 0 {
		t.Fatalf("anonymous request resolved owner %d, err=%v; want no owner and an authentication error", ownerID, err)
	}

	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	userID := mustTestUser(t, req.Context(), db, "session-owner-"+nonce+"@test.invalid")
	t.Cleanup(func() {
		db.ExecContext(req.Context(), `DELETE FROM sessions WHERE user_id=$1`, userID)
		db.ExecContext(req.Context(), `DELETE FROM users WHERE id=$1`, userID)
	})
	token, err := newSession(req.Context(), db, userID)
	if err != nil {
		t.Fatalf("newSession: %v", err)
	}
	authed := httptest.NewRequest("GET", "/api/status", nil)
	authed.AddCookie(&http.Cookie{Name: "jarvis_session", Value: token})
	ownerID, err = ownerFromSession(authed.Context(), db, authed)
	if err != nil || ownerID != userID {
		t.Fatalf("authenticated request resolved owner %d, err=%v; want %d", ownerID, err, userID)
	}
}

func TestCRMHTTPEndpointsRequireSession(t *testing.T) {
	db, err := OpenDB()
	if err != nil {
		t.Skipf("postgres not reachable (start with `docker compose up -d`): %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := InitSchema(context.Background(), db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}

	mux := http.NewServeMux()
	brain := &Brain{Model: "test"}
	registerHUD(mux, brain, &MemoryStore{}, db)
	registerChat(mux, brain, db)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/status", ""},
		{"GET", "/api/agent?name=oracle", ""},
		{"POST", "/api/oracle", `{"niche":"dentists"}`},
		{"POST", "/api/atlas", `{"lead":1}`},
		{"POST", "/api/pixel", `{"lead":1}`},
		{"POST", "/api/inbox", ""},
		{"POST", "/api/brief", ""},
		{"POST", "/api/send", `{"id":1}`},
		{"POST", "/api/approve", `{"id":1}`},
		{"POST", "/chat", `{"messages":[]}`},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, req)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d body=%q, want 401", response.Code, response.Body.String())
			}
		})
	}
}

func TestCRMQueriesCarryOwnerScope(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			query, err := strconv.Unquote(literal.Value)
			if err != nil || !crmSQL.MatchString(query) {
				return true
			}
			if !strings.Contains(strings.ToLower(query), "owner_id") {
				t.Errorf("%s:%d CRM query lacks owner_id scope: %q", path, fileSet.Position(literal.Pos()).Line, oneLine(query, 160))
			}
			return true
		})
	}
}

func TestCRMQueryGuardRecognizesUnscopedSQL(t *testing.T) {
	tests := []struct {
		query  string
		scoped bool
	}{
		{`SELECT * FROM leads`, false},
		{`SELECT * FROM leads WHERE owner_id=$1`, true},
		{`INSERT INTO outreach (lead_id, draft) VALUES ($1,$2)`, false},
		{`UPDATE contacts SET email=$1 WHERE owner_id=$2`, true},
	}
	for _, test := range tests {
		isCRM := crmSQL.MatchString(test.query)
		got := !isCRM || strings.Contains(strings.ToLower(test.query), "owner_id")
		if got != test.scoped {
			t.Errorf("guard(%q)=%v, want %v", test.query, got, test.scoped)
		}
	}
}
