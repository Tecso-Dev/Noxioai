package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = 1 * time.Hour
)

var errInvalidToken = errors.New("invalid_token")

// generateAuthToken returns a random 32-byte hex token, same shape as session tokens.
func generateAuthToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// tokenValid reports whether an auth_tokens row is still usable: not used, not expired.
func tokenValid(usedAt sql.NullTime, expiresAt time.Time, now time.Time) bool {
	return !usedAt.Valid && now.Before(expiresAt)
}

// issueAuthToken creates a single-use token for the given purpose ("verify" | "reset").
func issueAuthToken(ctx context.Context, db *sql.DB, userID int64, purpose string, ttl time.Duration) (string, error) {
	token, err := generateAuthToken()
	if err != nil {
		return "", err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO auth_tokens (token, user_id, purpose, expires_at) VALUES ($1,$2,$3,$4)`,
		token, userID, purpose, time.Now().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

// consumeAuthToken validates a single-use token for the given purpose and marks it used.
// Returns errInvalidToken for anything unusable (unknown, expired, already used).
func consumeAuthToken(ctx context.Context, db *sql.DB, token, purpose string) (int64, error) {
	var userID int64
	// single atomic claim: two concurrent requests can never both consume the same token
	err := db.QueryRowContext(ctx, `
		UPDATE auth_tokens SET used_at = now()
		WHERE token = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`,
		token, purpose).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errInvalidToken
	}
	if err != nil {
		return 0, err
	}
	return userID, nil
}

// sendAuthMail sends a text+HTML transactional email through the shared
// transport (Resend API in prod — aeza blocks SMTP — or SMTP locally).
func sendAuthMail(to, subject, text, html string) error {
	return deliverMail(to, subject, text, html)
}

func authMailHTML(heading, body, link, cta string) string {
	return fmt.Sprintf(`<!doctype html><html><body style="margin:0;padding:0;background:#f5f5f7;font-family:-apple-system,Segoe UI,Roboto,sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:40px 16px;">
<table width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;">
<tr><td style="padding:32px 32px 8px;"><span style="font-size:20px;font-weight:800;color:#111;">NOXIO<span style="color:#6d5ef8;">AI</span></span></td></tr>
<tr><td style="padding:8px 32px 0;"><h1 style="font-size:20px;color:#111;margin:16px 0;">%s</h1><p style="font-size:15px;color:#444;line-height:1.5;">%s</p></td></tr>
<tr><td style="padding:24px 32px;"><a href="%s" style="display:inline-block;background:#6d5ef8;color:#fff;text-decoration:none;font-weight:700;padding:12px 24px;border-radius:999px;">%s</a></td></tr>
<tr><td style="padding:0 32px 32px;"><p style="font-size:12px;color:#999;word-break:break-all;">%s</p></td></tr>
</table></td></tr></table></body></html>`, heading, body, link, cta, link)
}

// sendVerificationEmail emails the account-verification link.
func sendVerificationEmail(to, token string) error {
	link := fmt.Sprintf("%s/verify?token=%s", appBaseURL(), token)
	text := fmt.Sprintf("Welcome to NOXIOAI.\n\nConfirm your email address by visiting:\n%s\n\nThis link expires in 24 hours.", link)
	html := authMailHTML("Confirm your email", "Welcome to NOXIOAI — confirm your email address to activate your account.", link, "Verify email")
	return sendAuthMail(to, "Confirm your NOXIOAI email", text, html)
}

// sendResetEmail emails the password-reset link.
func sendResetEmail(to, token string) error {
	link := fmt.Sprintf("%s/reset?token=%s", appBaseURL(), token)
	text := fmt.Sprintf("Reset your NOXIOAI password by visiting:\n%s\n\nThis link expires in 1 hour. If you didn't request this, ignore this email.", link)
	html := authMailHTML("Reset your password", "We received a request to reset your NOXIOAI password. This link expires in 1 hour.", link, "Reset password")
	return sendAuthMail(to, "Reset your NOXIOAI password", text, html)
}

func writeAuthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// registerAuthEmail wires email verification and password-reset endpoints onto the mux.
func registerAuthEmail(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/auth/verify/confirm", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Token string `json:"token"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Token) == "" {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		userID, err := consumeAuthToken(r.Context(), db, req.Token, "verify")
		if errors.Is(err, errInvalidToken) {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if err != nil {
			http.Error(w, "could not verify email", http.StatusInternalServerError)
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET verified_at = now() WHERE id = $1`, userID); err != nil {
			http.Error(w, "could not verify email", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/reset/request", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Email string `json:"email"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		email := strings.TrimSpace(req.Email)
		if email != "" {
			var userID int64
			if err := db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE email = $1`, email).Scan(&userID); err == nil {
				if token, terr := issueAuthToken(r.Context(), db, userID, "reset", resetTokenTTL); terr == nil {
					go func(to, tok string) {
						if err := sendResetEmail(to, tok); err != nil {
							fmt.Fprintln(os.Stderr, "auth: send reset email:", err)
						}
					}(email, token)
				}
			}
		}
		// Always respond ok — never reveal whether the account exists.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Password) < 8 {
			http.Error(w, "invalid reset data", http.StatusBadRequest)
			return
		}
		userID, err := consumeAuthToken(r.Context(), db, req.Token, "reset")
		if errors.Is(err, errInvalidToken) {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET password_hash = $1 WHERE id = $2`, hash, userID); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		// Invalidate every existing session for this account.
		if _, err := db.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
}
