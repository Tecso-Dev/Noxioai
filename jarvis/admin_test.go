package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRequireAdmin proves the HUD's authorization gate denies anonymous and
// non-admin sessions and allows only a verified admin — with no live
// database, via the currentUserLookup seam.
func TestRequireAdmin(t *testing.T) {
	restore := currentUserLookup
	defer func() { currentUserLookup = restore }()

	cases := []struct {
		name   string
		lookup func(ctx context.Context, db *sql.DB, r *http.Request) (*User, error)
		wantOK bool
	}{
		{
			name:   "no session",
			lookup: func(context.Context, *sql.DB, *http.Request) (*User, error) { return nil, nil },
			wantOK: false,
		},
		{
			name: "non-admin verified user",
			lookup: func(context.Context, *sql.DB, *http.Request) (*User, error) {
				return &User{ID: 1, Verified: true, IsAdmin: false}, nil
			},
			wantOK: false,
		},
		{
			name: "unverified admin",
			lookup: func(context.Context, *sql.DB, *http.Request) (*User, error) {
				return &User{ID: 2, Verified: false, IsAdmin: true}, nil
			},
			wantOK: false,
		},
		{
			name: "session lookup error",
			lookup: func(context.Context, *sql.DB, *http.Request) (*User, error) {
				return nil, sql.ErrConnDone
			},
			wantOK: false,
		},
		{
			name: "verified admin",
			lookup: func(context.Context, *sql.DB, *http.Request) (*User, error) {
				return &User{ID: 3, Verified: true, IsAdmin: true}, nil
			},
			wantOK: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			currentUserLookup = tc.lookup
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/status", nil)

			user, ok := requireAdmin(w, r, nil)
			if ok != tc.wantOK {
				t.Fatalf("requireAdmin ok = %v, want %v", ok, tc.wantOK)
			}

			if tc.wantOK {
				if user == nil {
					t.Fatal("requireAdmin: expected non-nil user on success")
				}
				if w.Code != http.StatusOK {
					t.Fatalf("requireAdmin must not write a response on success; got status %d", w.Code)
				}
				return
			}

			if user != nil {
				t.Fatal("requireAdmin: expected nil user on denial")
			}
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["error"] != "forbidden" {
				t.Fatalf("body = %v, want error=forbidden", body)
			}
		})
	}
}

// TestRequireAdminNilDB proves the gate fails closed when the CRM database
// is unavailable — the HUD must never fall back to an authless mode.
func TestRequireAdminNilDB(t *testing.T) {
	restore := currentUserLookup
	defer func() { currentUserLookup = restore }()
	currentUserLookup = func(context.Context, *sql.DB, *http.Request) (*User, error) {
		return nil, sql.ErrConnDone
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	if _, ok := requireAdmin(w, r, nil); ok {
		t.Fatal("requireAdmin: must deny when db is nil")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
