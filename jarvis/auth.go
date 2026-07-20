package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	argon2Time    uint32 = 2
	argon2Memory  uint32 = 64 * 1024
	argon2Threads uint8  = 2
	argon2KeyLen  uint32 = 32
	argon2SaltLen        = 16
)

// User is the authenticated account data shared with future API handlers.
type User struct {
	ID               int64
	Email            string
	Username         string
	Name             string
	Locale           string
	IsAdmin          bool
	StripeCustomerID string
	Verified         bool
	SessionID        string
}

func hashPassword(pw string) (string, error) {
	pw = norm.NFC.String(pw)
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(pw), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func verifyPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil ||
		memory < 8*1024 || memory > 512*1024 || iterations == 0 || iterations > 10 || threads == 0 || threads > 16 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < argon2SaltLen || len(salt) > 64 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	canonical := norm.NFC.String(pw)
	actual := argon2.IDKey([]byte(canonical), salt, iterations, memory, threads, uint32(len(expected)))
	if subtle.ConstantTimeCompare(actual, expected) == 1 {
		return true
	}
	// Compatibility for accounts created before Unicode normalization was
	// introduced. A successful login is rehashed with the current policy.
	if canonical != pw {
		legacy := argon2.IDKey([]byte(pw), salt, iterations, memory, threads, uint32(len(expected)))
		return subtle.ConstantTimeCompare(legacy, expected) == 1
	}
	return false
}

func passwordNeedsRehash(encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return true
	}
	return parts[2] != fmt.Sprintf("v=%d", argon2.Version) ||
		parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", argon2Memory, argon2Time, argon2Threads)
}

var (
	dummyPasswordOnce sync.Once
	dummyPasswordHash string
)

func verifyAgainstDummyHash(password string) {
	dummyPasswordOnce.Do(func() {
		dummyPasswordHash, _ = hashPassword("this account does not exist 7f4c1d")
	})
	if dummyPasswordHash != "" {
		verifyPassword(dummyPasswordHash, password)
	}
}

type sessionOptions struct {
	Remember   bool
	AuthMethod string
}

func newSession(ctx context.Context, db *sql.DB, userID int64, r *http.Request, options sessionOptions) (string, time.Duration, error) {
	if db == nil {
		return "", 0, errors.New("database unavailable")
	}
	token, err := generateAuthToken()
	if err != nil {
		return "", 0, err
	}
	sessionID, err := generateAuthToken()
	if err != nil {
		return "", 0, err
	}
	lifetime := defaultSessionTTL
	if options.Remember {
		lifetime = rememberedSessionTTL
	}
	if options.AuthMethod == "" {
		options.AuthMethod = "password"
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO sessions
			(token, session_id, user_id, expires_at, last_seen_at, user_agent, ip_hint, remembered, auth_method)
		VALUES ($1,$2,$3,$4,now(),$5,$6,$7,$8)`,
		tokenDigest(token), sessionID, userID, time.Now().Add(lifetime), safeUserAgent(r.UserAgent()),
		requestIPHint(r), options.Remember, options.AuthMethod)
	if err != nil {
		return "", 0, err
	}
	return token, lifetime, nil
}

func setSessionCookie(w http.ResponseWriter, token string, lifetime time.Duration, remember bool) {
	cookie := &http.Cookie{
		Name: "jarvis_session", Value: token, Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode,
	}
	if remember {
		cookie.Expires = time.Now().Add(lifetime)
		cookie.MaxAge = int(lifetime.Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "jarvis_session", Value: "", Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1,
	})
}

func currentUser(ctx context.Context, db *sql.DB, r *http.Request) (*User, error) {
	cookie, err := r.Cookie("jarvis_session")
	if err != nil {
		return nil, nil
	}
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	var user User
	var storedToken string
	digest := tokenDigest(cookie.Value)
	err = db.QueryRowContext(ctx, `
		SELECT u.id, u.email, COALESCE(u.username,''), COALESCE(u.name,''), COALESCE(u.locale,'en'),
		       COALESCE(u.is_admin,false), COALESCE(u.stripe_customer_id,''), u.verified_at IS NOT NULL,
		       COALESCE(s.session_id,''), s.token
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE (s.token = $1 OR s.token = $2) AND s.expires_at > now()
		ORDER BY CASE WHEN s.token = $1 THEN 0 ELSE 1 END
		LIMIT 1`, digest, cookie.Value).
		Scan(&user.ID, &user.Email, &user.Username, &user.Name, &user.Locale, &user.IsAdmin,
			&user.StripeCustomerID, &user.Verified, &user.SessionID, &storedToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if storedToken == cookie.Value {
		// One-time compatibility migration for sessions created before token hashing.
		if _, err := db.ExecContext(ctx, `UPDATE sessions SET token = $1 WHERE token = $2`, digest, cookie.Value); err != nil {
			return nil, err
		}
	}
	if user.SessionID == "" {
		if sessionID, sessionErr := generateAuthToken(); sessionErr == nil {
			if result, updateErr := db.ExecContext(ctx, `
				UPDATE sessions SET session_id = $1 WHERE token = $2 AND session_id IS NULL`, sessionID, digest); updateErr == nil {
				if changed, _ := result.RowsAffected(); changed == 1 {
					user.SessionID = sessionID
				}
			}
		}
	}
	_, _ = db.ExecContext(ctx, `
		UPDATE sessions SET last_seen_at = now()
		WHERE token = $1 AND (last_seen_at IS NULL OR last_seen_at < now() - interval '5 minutes')`, digest)
	return &user, nil
}

func normalizeAuthLocale(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fa", "en", "tr", "ar":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "en"
	}
}

func sendVerificationForUser(db *sql.DB, userID int64, email string) {
	go func() {
		verifyToken, err := issueAuthToken(context.Background(), db, userID, "verify", verifyTokenTTL)
		if err != nil {
			log.Println("auth: issue verify token:", err)
			return
		}
		if err := sendVerificationEmail(email, verifyToken); err != nil {
			log.Println("auth: send verify email:", err)
		}
	}()
}

func knownLoginDevice(ctx context.Context, db *sql.DB, userID int64, userAgent string) (hasSessions, known bool) {
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM sessions WHERE user_id = $1),
		       EXISTS(SELECT 1 FROM sessions WHERE user_id = $1 AND user_agent = $2)`,
		userID, safeUserAgent(userAgent)).Scan(&hasSessions, &known)
	return err == nil && hasSessions, err == nil && known
}

func issueUserSession(w http.ResponseWriter, r *http.Request, db *sql.DB, userID int64, options sessionOptions) error {
	token, lifetime, err := newSession(r.Context(), db, userID, r, options)
	if err != nil {
		return err
	}
	setSessionCookie(w, token, lifetime, options.Remember)
	return nil
}

// registerAuth wires the NOXIOAI account API onto the serve mux.
func registerAuth(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/auth/signup", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "signup-ip", "", ratePolicy{Limit: 5, Window: time.Hour, Block: 10 * time.Minute}) {
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Email       string `json:"email"`
			Username    string `json:"username"`
			Password    string `json:"password"`
			Name        string `json:"name"`
			Locale      string `json:"locale"`
			AcceptTerms bool   `json:"accept_terms"`
		}
		if decodeAuthJSON(w, r, &req) != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_signup_data")
			return
		}
		if !req.AcceptTerms {
			writeAuthError(w, http.StatusBadRequest, "terms_acceptance_required")
			return
		}
		email, emailErr := normalizeEmail(req.Email)
		username, usernameErr := normalizeUsername(req.Username)
		if emailErr != nil || usernameErr != nil {
			code := "invalid_email"
			if usernameErr != nil {
				code = "invalid_username"
			}
			writeAuthError(w, http.StatusBadRequest, code)
			return
		}
		if !enforceAuthRateLimit(w, r, "signup-identity", email, ratePolicy{Limit: 3, Window: time.Hour, Block: 15 * time.Minute}) {
			return
		}
		if err := validatePassword(r.Context(), req.Password); err != nil {
			writeAuthError(w, http.StatusBadRequest, err.Error())
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			http.Error(w, "could not create account", http.StatusInternalServerError)
			return
		}
		var userID int64
		err = db.QueryRowContext(r.Context(), `
			INSERT INTO users
				(email, username, password_hash, name, locale, terms_accepted_at, privacy_accepted_at, legal_version)
			VALUES ($1,$2,$3,$4,$5,now(),now(),'2026-07-19')
			ON CONFLICT DO NOTHING
			RETURNING id`, email, username, hash, normalizeDisplayName(req.Name), normalizeAuthLocale(req.Locale)).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			// Enumeration-resistant response. For an existing, unverified email we
			// safely re-send confirmation; verified accounts and username conflicts
			// receive the same public response without changing the account.
			var verified bool
			if lookupErr := db.QueryRowContext(r.Context(), `
				SELECT id, verified_at IS NOT NULL FROM users WHERE lower(email) = $1`, email).
				Scan(&userID, &verified); lookupErr == nil && !verified {
				sendVerificationForUser(db, userID, email)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]bool{"ok": true, "verification_required": true})
			return
		}
		if err != nil {
			http.Error(w, "could not create account", http.StatusInternalServerError)
			return
		}
		recordAuthEvent(r.Context(), db, &userID, "signup_created", r)
		sendVerificationForUser(db, userID, email)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true, "verification_required": true})
	})

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "login-ip", "", ratePolicy{Limit: 30, Window: 10 * time.Minute, Block: time.Minute}) {
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Identifier string `json:"identifier"`
			Email      string `json:"email"`
			Password   string `json:"password"`
			Remember   bool   `json:"remember"`
		}
		if decodeAuthJSON(w, r, &req) != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		identifier := strings.ToLower(strings.TrimSpace(req.Identifier))
		if identifier == "" {
			identifier = strings.ToLower(strings.TrimSpace(req.Email))
		}
		failureKey := authRateKey("login-failure", r, identifier)
		if identifier == "" || req.Password == "" ||
			!enforceAuthRateLimit(w, r, "login-identity", identifier, ratePolicy{Limit: 8, Window: 10 * time.Minute, Block: 2 * time.Minute}) {
			if identifier == "" || req.Password == "" {
				writeAuthError(w, http.StatusUnauthorized, "invalid_credentials")
			}
			return
		}
		var user User
		var passwordHash string
		err := db.QueryRowContext(r.Context(), `
			SELECT id, email, COALESCE(username,''), password_hash, COALESCE(name,''), COALESCE(locale,'en'),
			       COALESCE(is_admin,false), verified_at IS NOT NULL
			FROM users WHERE lower(email) = $1 OR lower(username) = $1
			LIMIT 1`, identifier).
			Scan(&user.ID, &user.Email, &user.Username, &passwordHash, &user.Name, &user.Locale, &user.IsAdmin, &user.Verified)
		if errors.Is(err, sql.ErrNoRows) {
			verifyAgainstDummyHash(req.Password)
			time.Sleep(authLimiter.recordFailure(failureKey))
			recordAuthEvent(r.Context(), db, nil, "login_failed", r)
			writeAuthError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		if err != nil {
			http.Error(w, "could not log in", http.StatusInternalServerError)
			return
		}
		if !verifyPassword(passwordHash, req.Password) {
			time.Sleep(authLimiter.recordFailure(failureKey))
			recordAuthEvent(r.Context(), db, &user.ID, "login_failed", r)
			writeAuthError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		authLimiter.clearFailures(failureKey)
		if passwordNeedsRehash(passwordHash) {
			if upgraded, hashErr := hashPassword(req.Password); hashErr == nil {
				_, _ = db.ExecContext(r.Context(), `UPDATE users SET password_hash = $1 WHERE id = $2`, upgraded, user.ID)
			}
		}
		hadSessions, knownDevice := knownLoginDevice(r.Context(), db, user.ID, r.UserAgent())
		if err := issueUserSession(w, r, db, user.ID, sessionOptions{Remember: req.Remember, AuthMethod: "password"}); err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		recordAuthEvent(r.Context(), db, &user.ID, "login_succeeded", r)
		if hadSessions && !knownDevice {
			go func(email, agent, ip string) {
				if mailErr := sendNewLoginEmail(email, agent, ip); mailErr != nil {
					log.Printf("auth: send new login notice: %v", mailErr)
				}
			}(user.Email, safeUserAgent(r.UserAgent()), requestIPHint(r))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok": true, "name": user.Name, "locale": user.Locale, "verified": user.Verified,
		})
	})

	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) {
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		user, _ := currentUser(r.Context(), db, r)
		if cookie, err := r.Cookie("jarvis_session"); err == nil {
			if _, err := db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token = $1 OR token = $2`, tokenDigest(cookie.Value), cookie.Value); err != nil {
				http.Error(w, "could not log out", http.StatusInternalServerError)
				return
			}
		}
		clearSessionCookie(w)
		if user != nil {
			recordAuthEvent(r.Context(), db, &user.ID, "logout", r)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/logout-all", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) {
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := db.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id = $1`, user.ID); err != nil {
			http.Error(w, "could not end sessions", http.StatusInternalServerError)
			return
		}
		clearSessionCookie(w)
		recordAuthEvent(r.Context(), db, &user.ID, "all_sessions_revoked", r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil {
			http.Error(w, "could not get current user", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"email": user.Email, "username": user.Username, "name": user.Name, "locale": user.Locale, "is_admin": user.IsAdmin,
			"verified": user.Verified,
		})
	})

	mux.HandleFunc("GET /api/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		rows, err := db.QueryContext(r.Context(), `
			SELECT COALESCE(session_id,''), created_at, COALESCE(last_seen_at,created_at),
			       COALESCE(user_agent,''), COALESCE(ip_hint,''), COALESCE(remembered,false),
			       COALESCE(auth_method,'password'), expires_at
			FROM sessions WHERE user_id = $1 AND expires_at > now()
			ORDER BY COALESCE(last_seen_at,created_at) DESC`, user.ID)
		if err != nil {
			http.Error(w, "could not list sessions", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		type sessionView struct {
			ID         string    `json:"id"`
			CreatedAt  time.Time `json:"created_at"`
			LastSeenAt time.Time `json:"last_seen_at"`
			UserAgent  string    `json:"user_agent"`
			IPHint     string    `json:"ip_hint"`
			Remembered bool      `json:"remembered"`
			AuthMethod string    `json:"auth_method"`
			ExpiresAt  time.Time `json:"expires_at"`
			Current    bool      `json:"current"`
		}
		sessions := make([]sessionView, 0, 4)
		for rows.Next() {
			var session sessionView
			if err := rows.Scan(&session.ID, &session.CreatedAt, &session.LastSeenAt, &session.UserAgent,
				&session.IPHint, &session.Remembered, &session.AuthMethod, &session.ExpiresAt); err != nil {
				http.Error(w, "could not list sessions", http.StatusInternalServerError)
				return
			}
			session.Current = session.ID != "" && session.ID == user.SessionID
			sessions = append(sessions, session)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"sessions": sessions})
	})

	mux.HandleFunc("DELETE /api/auth/sessions/{sessionID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) {
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sessionID := r.PathValue("sessionID")
		if len(sessionID) != 64 {
			writeAuthError(w, http.StatusBadRequest, "invalid_session")
			return
		}
		if _, err := hex.DecodeString(sessionID); err != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_session")
			return
		}
		result, err := db.ExecContext(r.Context(), `DELETE FROM sessions WHERE session_id = $1 AND user_id = $2`, sessionID, user.ID)
		if err != nil {
			http.Error(w, "could not end session", http.StatusInternalServerError)
			return
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			recordAuthEvent(r.Context(), db, &user.ID, "session_revoked", r)
		}
		if sessionID == user.SessionID {
			clearSessionCookie(w)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	registerAuthEmail(mux, db)
	registerPasskeys(mux, db)
	registerGoogleAuth(mux, db)
}
