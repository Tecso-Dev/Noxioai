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
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Time    uint32 = 1
	argon2Memory  uint32 = 64 * 1024
	argon2Threads uint8  = 4
	argon2KeyLen  uint32 = 32
	argon2SaltLen        = 16

	sessionLifetime = 30 * 24 * time.Hour
)

// User is the authenticated account data shared with future API handlers.
type User struct {
	ID               int64
	Email            string
	Name             string
	Locale           string
	IsAdmin          bool
	StripeCustomerID string
}

func hashPassword(pw string) (string, error) {
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
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" ||
		parts[2] != fmt.Sprintf("v=%d", argon2.Version) ||
		parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", argon2Memory, argon2Time, argon2Threads) {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != argon2SaltLen {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != int(argon2KeyLen) {
		return false
	}
	actual := argon2.IDKey([]byte(pw), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func newSession(ctx context.Context, db *sql.DB, userID int64) (string, error) {
	if db == nil {
		return "", errors.New("database unavailable")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	_, err := db.ExecContext(ctx, `
		INSERT INTO sessions (token, user_id, expires_at) VALUES ($1,$2,$3)`,
		token, userID, time.Now().Add(sessionLifetime))
	if err != nil {
		return "", err
	}
	return token, nil
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
	err = db.QueryRowContext(ctx, `
		SELECT u.id, u.email, COALESCE(u.name,''), COALESCE(u.locale,'en'), COALESCE(u.is_admin,false), COALESCE(u.stripe_customer_id,'')
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > now()`, cookie.Value).
		Scan(&user.ID, &user.Email, &user.Name, &user.Locale, &user.IsAdmin, &user.StripeCustomerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// registerAuth wires the NOXIOAI account API onto the serve mux.
func registerAuth(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/auth/signup", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Email) == "" || len(req.Password) < 8 {
			http.Error(w, "invalid signup data", http.StatusBadRequest)
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			http.Error(w, "could not create account", http.StatusInternalServerError)
			return
		}
		var userID int64
		err = db.QueryRowContext(r.Context(), `
			INSERT INTO users (email, password_hash, name) VALUES ($1,$2,$3)
			ON CONFLICT (email) DO NOTHING
			RETURNING id`, req.Email, hash, req.Name).Scan(&userID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, "could not create account", http.StatusInternalServerError)
			return
		}
		token, err := newSession(r.Context(), db, userID)
		if err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: "jarvis_session", Value: token, Path: "/", HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(sessionLifetime),
			MaxAge: int(sessionLifetime.Seconds()),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		var user User
		var passwordHash string
		err := db.QueryRowContext(r.Context(), `
			SELECT id, email, password_hash, COALESCE(name,''), COALESCE(locale,'en'), COALESCE(is_admin,false)
			FROM users WHERE email = $1`, req.Email).
			Scan(&user.ID, &user.Email, &passwordHash, &user.Name, &user.Locale, &user.IsAdmin)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !verifyPassword(passwordHash, req.Password)) {
			http.Error(w, "invalid email or password", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "could not log in", http.StatusInternalServerError)
			return
		}
		token, err := newSession(r.Context(), db, user.ID)
		if err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: "jarvis_session", Value: token, Path: "/", HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(sessionLifetime),
			MaxAge: int(sessionLifetime.Seconds()),
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "name": user.Name, "locale": user.Locale})
	})

	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		if cookie, err := r.Cookie("jarvis_session"); err == nil {
			if _, err := db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token = $1`, cookie.Value); err != nil {
				http.Error(w, "could not log out", http.StatusInternalServerError)
				return
			}
		}
		http.SetCookie(w, &http.Cookie{
			Name: "jarvis_session", Value: "", Path: "/", HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1,
		})
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
			"email": user.Email, "name": user.Name, "locale": user.Locale, "is_admin": user.IsAdmin,
		})
	})
}
