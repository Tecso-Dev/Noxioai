package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// currentUserLookup is the seam admin_test.go overrides to exercise
// requireAdmin without a live database.
var currentUserLookup = currentUser

// resolveAdmin resolves the requester's session user and reports whether
// they are a verified super-admin (users.is_admin = true). It never writes
// to w — callers decide how a non-admin request should be answered.
func resolveAdmin(r *http.Request, db *sql.DB) (*User, bool) {
	user, err := currentUserLookup(r.Context(), db, r)
	if err != nil || user == nil || !user.Verified || !user.IsAdmin {
		return nil, false
	}
	return user, true
}

// requireAdmin is the single authorization gate for the JARVIS command-center
// HUD's API and agent-control endpoints. Only a valid session whose user has
// is_admin = true may pass; anonymous requests and non-admin accounts alike
// get a uniform 403. The gate is is_admin, never a secret URL.
func requireAdmin(w http.ResponseWriter, r *http.Request, db *sql.DB) (*User, bool) {
	user, ok := resolveAdmin(r, db)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
		return nil, false
	}
	return user, true
}
