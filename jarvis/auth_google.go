package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	googleStateCookieName = "jarvis_google_state"
	googleStatePath       = "/api/auth/google"
	googleStateTTL        = 10 * time.Minute
	googleAuthEndpoint    = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint   = "https://oauth2.googleapis.com/token"
)

var googleHTTPClient = &http.Client{Timeout: 8 * time.Second}

// googleIDTokenClaims is the subset of Google's ID token payload we rely on.
// Signature verification is intentionally skipped: the token is read straight
// back from Google's token endpoint over TLS (never from client input), so the
// channel itself is the trust anchor. We still verify iss/aud/exp/email_verified.
type googleIDTokenClaims struct {
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Name          string `json:"name"`
	Exp           int64  `json:"exp"`
}

func (c googleIDTokenClaims) verifiedEmail() bool {
	switch v := c.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

var errInvalidGoogleToken = errors.New("invalid_google_token")

// parseGoogleIDToken decodes (without verifying the signature) the claims
// segment of a Google-issued JWT.
func parseGoogleIDToken(idToken string) (*googleIDTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 || parts[1] == "" {
		return nil, errInvalidGoogleToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidGoogleToken
	}
	var claims googleIDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errInvalidGoogleToken
	}
	return &claims, nil
}

// validateGoogleClaims enforces the claims a same-origin OAuth code-flow
// callback must check: issuer, audience, expiry, and a verified email.
func validateGoogleClaims(claims *googleIDTokenClaims, clientID string, now time.Time) error {
	if claims == nil || clientID == "" {
		return errInvalidGoogleToken
	}
	if claims.Iss != "https://accounts.google.com" && claims.Iss != "accounts.google.com" {
		return errInvalidGoogleToken
	}
	if claims.Aud != clientID {
		return errInvalidGoogleToken
	}
	if claims.Exp <= now.Unix() {
		return errInvalidGoogleToken
	}
	if !claims.verifiedEmail() {
		return errInvalidGoogleToken
	}
	if claims.Sub == "" || claims.Email == "" {
		return errInvalidGoogleToken
	}
	return nil
}

// googleAccountAction is the outcome of matching an incoming Google identity
// against existing accounts.
type googleAccountAction int

const (
	googleActionLogin googleAccountAction = iota
	googleActionLink
	googleActionSignup
)

// decideGoogleAccountAction is the pure account-matching decision: a sub match
// always wins (it is the durable identity), an email match without a sub
// links the existing password/passkey account, otherwise a new account is
// created.
func decideGoogleAccountAction(subMatched, emailMatched bool) googleAccountAction {
	switch {
	case subMatched:
		return googleActionLogin
	case emailMatched:
		return googleActionLink
	default:
		return googleActionSignup
	}
}

func googleAccountActionEvent(action googleAccountAction) string {
	switch action {
	case googleActionLogin:
		return "auth.google.login"
	case googleActionLink:
		return "auth.google.link"
	default:
		return "auth.google.signup"
	}
}

func googleRedirectURL() string {
	return envOr("JARVIS_GOOGLE_REDIRECT_URL", "https://noxioai.com/api/auth/google/callback")
}

func setGoogleStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name: googleStateCookieName, Value: state, Path: googleStatePath,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		Expires: time.Now().Add(googleStateTTL), MaxAge: int(googleStateTTL.Seconds()),
	})
}

func clearGoogleStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: googleStateCookieName, Value: "", Path: googleStatePath,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		Expires: time.Unix(1, 0), MaxAge: -1,
	})
}

// verifyState does a constant-time comparison between the state cookie and
// the state query parameter Google echoed back.
func verifyState(cookieValue, paramValue string) bool {
	if cookieValue == "" || paramValue == "" || len(cookieValue) != len(paramValue) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieValue), []byte(paramValue)) == 1
}

func redirectGoogleError(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login?error=google", http.StatusFound)
}

// exchangeGoogleCode swaps an authorization code for an ID token at Google's
// token endpoint. The access/refresh tokens in the response are discarded —
// only the ID token (used once, to read identity claims) is returned.
func exchangeGoogleCode(ctx context.Context, clientID, clientSecret, redirectURI, code string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := googleHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google token exchange: %s", resp.Status)
	}
	var payload struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}
	if payload.IDToken == "" {
		return "", errors.New("google token response missing id_token")
	}
	return payload.IDToken, nil
}

// resolveGoogleUser maps a validated Google identity onto a users row,
// creating or linking an account as needed.
//
// ponytail: the email-match-then-insert path has a narrow TOCTOU window if two
// signups for the same brand-new email race; the second loses to the unique
// index and the request fails closed to the login-error redirect. Upgrade to
// a serializable transaction if concurrent first-time Google signups for the
// same address become a real occurrence.
func resolveGoogleUser(ctx context.Context, db *sql.DB, claims *googleIDTokenClaims) (int64, string, error) {
	email := strings.ToLower(strings.TrimSpace(claims.Email))

	var subUserID int64
	subErr := db.QueryRowContext(ctx, `SELECT id FROM users WHERE google_sub = $1`, claims.Sub).Scan(&subUserID)
	if subErr != nil && !errors.Is(subErr, sql.ErrNoRows) {
		return 0, "", subErr
	}
	subMatched := subErr == nil

	var emailUserID int64
	emailErr := db.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email) = $1`, email).Scan(&emailUserID)
	if emailErr != nil && !errors.Is(emailErr, sql.ErrNoRows) {
		return 0, "", emailErr
	}
	emailMatched := emailErr == nil

	switch action := decideGoogleAccountAction(subMatched, emailMatched); action {
	case googleActionLogin:
		return subUserID, googleAccountActionEvent(action), nil
	case googleActionLink:
		if _, err := db.ExecContext(ctx, `
			UPDATE users SET google_sub = $1, verified_at = COALESCE(verified_at, now()) WHERE id = $2`,
			claims.Sub, emailUserID); err != nil {
			return 0, "", err
		}
		return emailUserID, googleAccountActionEvent(action), nil
	default:
		randomSecret, err := generateAuthToken()
		if err != nil {
			return 0, "", err
		}
		// Google-only accounts never get a usable password: this hash is a
		// throwaway 32-byte random secret nobody knows, satisfying the
		// NOT NULL column without giving the account a real password login.
		hash, err := hashPassword(randomSecret)
		if err != nil {
			return 0, "", err
		}
		var userID int64
		err = db.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, name, locale, google_sub, verified_at, terms_accepted_at, privacy_accepted_at)
			VALUES ($1,$2,$3,'en',$4,now(),now(),now())
			ON CONFLICT DO NOTHING
			RETURNING id`, email, hash, normalizeDisplayName(claims.Name), claims.Sub).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", errors.New("account already exists")
		}
		if err != nil {
			return 0, "", err
		}
		return userID, googleAccountActionEvent(action), nil
	}
}

// registerGoogleAuth wires the "Continue with Google" OAuth 2.0 server-side
// code flow onto the mux: a start endpoint that redirects to Google, and a
// callback that exchanges the code, validates the identity, and signs the
// user in through the same session path password login uses.
func registerGoogleAuth(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/auth/google/start", func(w http.ResponseWriter, r *http.Request) {
		clientID := strings.TrimSpace(os.Getenv("JARVIS_GOOGLE_CLIENT_ID"))
		if clientID == "" {
			http.Error(w, "Google sign-in is not configured", http.StatusNotFound)
			return
		}
		if !enforceAuthRateLimit(w, r, "google-start-ip", "", ratePolicy{Limit: 30, Window: time.Hour, Block: 5 * time.Minute}) {
			return
		}
		state, err := generateAuthToken()
		if err != nil {
			http.Error(w, "could not start Google sign-in", http.StatusInternalServerError)
			return
		}
		setGoogleStateCookie(w, state)
		values := url.Values{
			"client_id":     {clientID},
			"redirect_uri":  {googleRedirectURL()},
			"response_type": {"code"},
			"scope":         {"openid email profile"},
			"state":         {state},
			"prompt":        {"select_account"},
		}
		http.Redirect(w, r, googleAuthEndpoint+"?"+values.Encode(), http.StatusFound)
	})

	mux.HandleFunc("GET /api/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		if !enforceAuthRateLimit(w, r, "google-callback-ip", "", ratePolicy{Limit: 30, Window: time.Hour, Block: 5 * time.Minute}) {
			return
		}
		var cookieState string
		if cookie, err := r.Cookie(googleStateCookieName); err == nil {
			cookieState = cookie.Value
		}
		clearGoogleStateCookie(w)
		if !verifyState(cookieState, r.URL.Query().Get("state")) {
			redirectGoogleError(w, r)
			return
		}
		if db == nil {
			redirectGoogleError(w, r)
			return
		}
		clientID := strings.TrimSpace(os.Getenv("JARVIS_GOOGLE_CLIENT_ID"))
		clientSecret := strings.TrimSpace(os.Getenv("JARVIS_GOOGLE_CLIENT_SECRET"))
		code := r.URL.Query().Get("code")
		if clientID == "" || clientSecret == "" || code == "" {
			redirectGoogleError(w, r)
			return
		}
		idToken, err := exchangeGoogleCode(r.Context(), clientID, clientSecret, googleRedirectURL(), code)
		if err != nil {
			redirectGoogleError(w, r)
			return
		}
		claims, err := parseGoogleIDToken(idToken)
		if err != nil {
			redirectGoogleError(w, r)
			return
		}
		if err := validateGoogleClaims(claims, clientID, time.Now()); err != nil {
			redirectGoogleError(w, r)
			return
		}
		userID, event, err := resolveGoogleUser(r.Context(), db, claims)
		if err != nil {
			redirectGoogleError(w, r)
			return
		}
		if err := issueUserSession(w, r, db, userID, sessionOptions{AuthMethod: "google"}); err != nil {
			redirectGoogleError(w, r)
			return
		}
		recordAuthEvent(r.Context(), db, &userID, event, r)
		http.Redirect(w, r, "/app", http.StatusFound)
	})
}
