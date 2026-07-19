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
	"time"
)

const (
	verifyTokenTTL = 24 * time.Hour
	resetTokenTTL  = 1 * time.Hour
)

var errInvalidToken = errors.New("invalid_token")
var minimumResetResponseDelay = 350 * time.Millisecond

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
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `
		UPDATE auth_tokens SET used_at = now()
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`, userID, purpose); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO auth_tokens (token, user_id, purpose, expires_at) VALUES ($1,$2,$3,$4)`,
		tokenDigest(token), userID, purpose, time.Now().Add(ttl)); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

// consumeAuthToken validates a single-use token for the given purpose and marks it used.
// Returns errInvalidToken for anything unusable (unknown, expired, already used).
func consumeAuthToken(ctx context.Context, db queryExecer, token, purpose string) (int64, error) {
	var userID int64
	// single atomic claim: two concurrent requests can never both consume the same token
	err := db.QueryRowContext(ctx, `
		UPDATE auth_tokens SET used_at = now()
		WHERE (token = $1 OR token = $2) AND purpose = $3 AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`,
		tokenDigest(token), token, purpose).Scan(&userID)
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
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"></head><body style="margin:0;padding:0;background:#f4f4f8;font-family:-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="center" style="padding:40px 16px;">
<table width="520" cellpadding="0" cellspacing="0" role="presentation" style="max-width:100%%;background:#ffffff;border-radius:16px;overflow:hidden;box-shadow:0 1px 4px rgba(20,20,40,0.08);">
<tr><td style="height:5px;background:#b39868;background:linear-gradient(90deg,#d4bf94,#b39868);font-size:0;line-height:0;">&nbsp;</td></tr>
<tr><td align="center" style="padding:44px 40px 8px;">
  <img src="https://noxioai.com/brand/noxioai-logo.png" width="60" height="60" alt="NOXIOAI" style="display:block;margin:0 auto 14px;border-radius:50%%;border:1px solid #e6ddc9;">
  <span style="font-size:22px;font-weight:800;letter-spacing:-0.02em;color:#111;">NOXIO<span style="color:#b39868;">AI</span></span>
</td></tr>
<tr><td align="center" style="padding:12px 44px 0;">
  <h1 style="font-size:21px;color:#111;margin:14px 0 8px;">%s</h1>
  <p style="font-size:15px;color:#4a4a5a;line-height:1.65;margin:0;">%s</p>
</td></tr>
<tr><td align="center" style="padding:32px 44px 12px;">
  <a href="%s" bgcolor="#d4bf94" style="display:inline-block;background:#d4bf94;background:linear-gradient(120deg,#d4bf94,#b39868);color:#111111;text-decoration:none;font-weight:700;font-size:15px;padding:14px 34px;border-radius:999px;box-shadow:0 4px 14px rgba(179,152,104,0.35);">%s</a>
</td></tr>
<tr><td align="center" style="padding:8px 44px 10px;">
  <p style="font-size:12px;color:#9a95b0;line-height:1.6;">Button not working? Copy this link:<br><span style="word-break:break-all;color:#b39868;">%s</span></p>
</td></tr>
<tr><td style="padding:22px 44px 34px;border-top:1px solid #eeeef4;" align="center">
  <p style="font-size:12px;color:#9a95b0;margin:0;line-height:1.7;">NOXIOAI — AI employees that work while you sleep.<br>
  <a href="https://noxioai.com" style="color:#b39868;text-decoration:none;">noxioai.com</a> &nbsp;·&nbsp; <a href="mailto:hi@noxioai.com" style="color:#b39868;text-decoration:none;">hi@noxioai.com</a><br>
  © 2026 NOXIOAI. All rights reserved.</p>
</td></tr>
</table>
<p style="font-size:11px;color:#b6b2c6;margin:18px 0 0;">You received this because of an account action at noxioai.com.</p>
</td></tr></table></body></html>`, heading, body, link, cta, link)
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

func sendNewLoginEmail(to, userAgent, ipHint string) error {
	when := time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")
	body := fmt.Sprintf("A new device signed in to your NOXIOAI account at %s. Device: %s. Network: %s. If this was not you, reset your password and end active sessions from Account Security.", when, userAgent, ipHint)
	text := body + "\n\nAccount security: " + appBaseURL() + "/account"
	html := authMailHTML("New sign-in to your account", body, appBaseURL()+"/account", "Review sessions")
	return sendAuthMail(to, "New sign-in to your NOXIOAI account", text, html)
}

func sendPasswordChangedEmail(to string) error {
	body := "Your NOXIOAI password was changed and all existing sessions were ended. If you did not make this change, contact hi@noxioai.com immediately."
	text := body + "\n\nLog in: " + appBaseURL() + "/login"
	html := authMailHTML("Your password was changed", body, appBaseURL()+"/login", "Log in securely")
	return sendAuthMail(to, "Your NOXIOAI password was changed", text, html)
}

func sendPasskeyChangedEmail(to, action string) error {
	body := fmt.Sprintf("A passkey was %s on your NOXIOAI account. If you did not make this change, review your active sessions and reset your password immediately.", action)
	text := body + "\n\nAccount security: " + appBaseURL() + "/account"
	html := authMailHTML("Passkey security changed", body, appBaseURL()+"/account", "Review account security")
	return sendAuthMail(to, "Passkey security changed on your NOXIOAI account", text, html)
}

func writeAuthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// registerAuthEmail wires email verification and password-reset endpoints onto the mux.
func registerAuthEmail(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/auth/verify/confirm", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "verify-ip", "", ratePolicy{Limit: 20, Window: time.Hour, Block: 5 * time.Minute}) {
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Token string `json:"token"`
		}
		if decodeAuthJSON(w, r, &req) != nil || len(req.Token) != 64 {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if _, err := hex.DecodeString(req.Token); err != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if !enforceAuthRateLimit(w, r, "verify-token", req.Token, ratePolicy{Limit: 5, Window: 15 * time.Minute, Block: 15 * time.Minute}) {
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "could not verify email", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		userID, err := consumeAuthToken(r.Context(), tx, req.Token, "verify")
		if errors.Is(err, errInvalidToken) {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if err != nil {
			http.Error(w, "could not verify email", http.StatusInternalServerError)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET verified_at = now() WHERE id = $1`, userID); err != nil {
			http.Error(w, "could not verify email", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "could not verify email", http.StatusInternalServerError)
			return
		}
		recordAuthEvent(r.Context(), db, &userID, "email_verified", r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/reset/request", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		defer func() {
			if remaining := minimumResetResponseDelay - time.Since(started); remaining > 0 {
				time.Sleep(remaining)
			}
		}()
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "reset-request-ip", "", ratePolicy{Limit: 5, Window: 15 * time.Minute, Block: 15 * time.Minute}) {
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Email string `json:"email"`
		}
		if decodeAuthJSON(w, r, &req) != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		email, _ := normalizeEmail(req.Email)
		if email != "" && enforceAuthRateLimit(w, r, "reset-request-identity", email, ratePolicy{Limit: 3, Window: time.Hour, Block: 30 * time.Minute}) {
			var userID int64
			if err := db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE lower(email) = $1`, email).Scan(&userID); err == nil {
				if token, terr := issueAuthToken(r.Context(), db, userID, "reset", resetTokenTTL); terr == nil {
					go func(to, tok string) {
						if err := sendResetEmail(to, tok); err != nil {
							fmt.Fprintln(os.Stderr, "auth: send reset email:", err)
						}
					}(email, token)
				}
				recordAuthEvent(r.Context(), db, &userID, "password_reset_requested", r)
			} else {
				recordAuthEvent(r.Context(), db, nil, "password_reset_requested", r)
			}
		}
		// Always respond ok — never reveal whether the account exists.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "reset-confirm-ip", "", ratePolicy{Limit: 10, Window: time.Hour, Block: 15 * time.Minute}) {
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if decodeAuthJSON(w, r, &req) != nil || len(req.Token) != 64 {
			writeAuthError(w, http.StatusBadRequest, "invalid_reset_data")
			return
		}
		if _, err := hex.DecodeString(req.Token); err != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if !enforceAuthRateLimit(w, r, "reset-confirm-token", req.Token, ratePolicy{Limit: 5, Window: 15 * time.Minute, Block: 30 * time.Minute}) {
			return
		}
		if err := validatePassword(r.Context(), req.Password); err != nil {
			writeAuthError(w, http.StatusBadRequest, err.Error())
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		userID, err := consumeAuthToken(r.Context(), tx, req.Token, "reset")
		if errors.Is(err, errInvalidToken) {
			writeAuthError(w, http.StatusBadRequest, "invalid_token")
			return
		}
		if err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		var email string
		if err := tx.QueryRowContext(r.Context(), `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET password_hash = $1 WHERE id = $2`, hash, userID); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		// Invalidate every existing session for this account.
		if _, err := tx.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			UPDATE auth_tokens SET used_at = now()
			WHERE user_id = $1 AND purpose = 'reset' AND used_at IS NULL`, userID); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "could not reset password", http.StatusInternalServerError)
			return
		}
		clearSessionCookie(w)
		recordAuthEvent(r.Context(), db, &userID, "password_reset_completed", r)
		go func() {
			if mailErr := sendPasswordChangedEmail(email); mailErr != nil {
				fmt.Fprintln(os.Stderr, "auth: send password changed email:", mailErr)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
}
