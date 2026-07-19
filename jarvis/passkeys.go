package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const passkeyChallengeTTL = 5 * time.Minute

type passkeyManager struct {
	db       *sql.DB
	webauthn *webauthn.WebAuthn
	cipher   cipher.AEAD
}

type passkeyUser struct {
	ID          int64
	Email       string
	Username    string
	Name        string
	Verified    bool
	Handle      []byte
	Credentials []webauthn.Credential
}

func (user *passkeyUser) WebAuthnID() []byte { return user.Handle }
func (user *passkeyUser) WebAuthnName() string {
	if user.Username != "" {
		return user.Username
	}
	return user.Email
}
func (user *passkeyUser) WebAuthnDisplayName() string {
	if user.Name != "" {
		return user.Name
	}
	return user.WebAuthnName()
}
func (user *passkeyUser) WebAuthnCredentials() []webauthn.Credential { return user.Credentials }

func authDataKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv("JARVIS_AUTH_DATA_KEY"))
	if raw == "" {
		return nil, errors.New("JARVIS_AUTH_DATA_KEY is not configured")
	}
	for _, decoder := range []func(string) ([]byte, error){base64.StdEncoding.DecodeString, base64.RawStdEncoding.DecodeString, hex.DecodeString} {
		if key, err := decoder(raw); err == nil && len(key) == 32 {
			return key, nil
		}
	}
	return nil, errors.New("JARVIS_AUTH_DATA_KEY must encode exactly 32 bytes")
}

func newPasskeyManager(db *sql.DB) (*passkeyManager, error) {
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	key, err := authDataKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(appBaseURL())
	if err != nil || base.Hostname() == "" || base.Scheme == "" {
		return nil, errors.New("APP_BASE_URL must be an absolute URL for WebAuthn")
	}
	rpID := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ID"))
	if rpID == "" {
		rpID = base.Hostname()
	}
	origins := []string{base.Scheme + "://" + base.Host}
	if configured := strings.TrimSpace(os.Getenv("WEBAUTHN_ORIGINS")); configured != "" {
		origins = origins[:0]
		for _, origin := range strings.Split(configured, ",") {
			if origin = strings.TrimSpace(strings.TrimSuffix(origin, "/")); origin != "" {
				origins = append(origins, origin)
			}
		}
	}
	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "NOXIOAI",
		RPID:          rpID,
		RPOrigins:     origins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: passkeyChallengeTTL, TimeoutUVD: passkeyChallengeTTL},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: passkeyChallengeTTL, TimeoutUVD: passkeyChallengeTTL},
		},
	})
	if err != nil {
		return nil, err
	}
	return &passkeyManager{db: db, webauthn: webAuthn, cipher: aead}, nil
}

func (manager *passkeyManager) seal(plaintext []byte, associatedData string) ([]byte, error) {
	nonce := make([]byte, manager.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return manager.cipher.Seal(nonce, nonce, plaintext, []byte(associatedData)), nil
}

func (manager *passkeyManager) open(ciphertext []byte, associatedData string) ([]byte, error) {
	nonceSize := manager.cipher.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("encrypted auth data is truncated")
	}
	return manager.cipher.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], []byte(associatedData))
}

func passkeyCredentialAAD(userID int64, credentialID []byte) string {
	return fmt.Sprintf("passkey:%d:%s", userID, base64.RawURLEncoding.EncodeToString(credentialID))
}

func (manager *passkeyManager) ensureUserHandle(ctx context.Context, userID int64) ([]byte, error) {
	var handle []byte
	err := manager.db.QueryRowContext(ctx, `SELECT webauthn_id FROM users WHERE id = $1`, userID).Scan(&handle)
	if err != nil {
		return nil, err
	}
	if len(handle) != 0 {
		return handle, nil
	}
	candidate := make([]byte, 32)
	if _, err := rand.Read(candidate); err != nil {
		return nil, err
	}
	err = manager.db.QueryRowContext(ctx, `
		UPDATE users SET webauthn_id = $1
		WHERE id = $2 AND webauthn_id IS NULL
		RETURNING webauthn_id`, candidate, userID).Scan(&handle)
	if errors.Is(err, sql.ErrNoRows) {
		err = manager.db.QueryRowContext(ctx, `SELECT webauthn_id FROM users WHERE id = $1`, userID).Scan(&handle)
	}
	return handle, err
}

func (manager *passkeyManager) loadUser(ctx context.Context, userID int64, ensureHandle bool) (*passkeyUser, error) {
	var user passkeyUser
	err := manager.db.QueryRowContext(ctx, `
		SELECT id, email, COALESCE(username,''), COALESCE(name,''), verified_at IS NOT NULL,
		       COALESCE(webauthn_id, ''::bytea)
		FROM users WHERE id = $1`, userID).
		Scan(&user.ID, &user.Email, &user.Username, &user.Name, &user.Verified, &user.Handle)
	if err != nil {
		return nil, err
	}
	if ensureHandle && len(user.Handle) == 0 {
		if user.Handle, err = manager.ensureUserHandle(ctx, userID); err != nil {
			return nil, err
		}
	}
	rows, err := manager.db.QueryContext(ctx, `
		SELECT credential_id, credential_data FROM passkeys WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var credentialID, encrypted []byte
		if err := rows.Scan(&credentialID, &encrypted); err != nil {
			return nil, err
		}
		plaintext, err := manager.open(encrypted, passkeyCredentialAAD(userID, credentialID))
		if err != nil {
			return nil, err
		}
		var credential webauthn.Credential
		if err := json.Unmarshal(plaintext, &credential); err != nil {
			return nil, err
		}
		user.Credentials = append(user.Credentials, credential)
	}
	return &user, rows.Err()
}

func (manager *passkeyManager) loadUserByHandle(ctx context.Context, rawID, handle []byte) (webauthn.User, error) {
	var userID int64
	err := manager.db.QueryRowContext(ctx, `
		SELECT u.id FROM users u
		JOIN passkeys p ON p.user_id = u.id
		WHERE u.webauthn_id = $1 AND p.credential_id = $2 AND u.verified_at IS NOT NULL`, handle, rawID).Scan(&userID)
	if err != nil {
		return nil, err
	}
	return manager.loadUser(ctx, userID, false)
}

func setPasskeyChallengeCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: "noxio_passkey_challenge", Value: token, Path: "/api/auth/passkeys", HttpOnly: true,
		Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: int(passkeyChallengeTTL.Seconds()),
		Expires: time.Now().Add(passkeyChallengeTTL),
	})
}

func clearPasskeyChallengeCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: "noxio_passkey_challenge", Value: "", Path: "/api/auth/passkeys", HttpOnly: true,
		Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0),
	})
}

func (manager *passkeyManager) saveChallenge(ctx context.Context, w http.ResponseWriter, userID *int64, purpose string, session *webauthn.SessionData, remember bool) error {
	encoded, err := json.Marshal(session)
	if err != nil {
		return err
	}
	sealed, err := manager.seal(encoded, "webauthn-challenge:"+purpose)
	if err != nil {
		return err
	}
	token, err := generateAuthToken()
	if err != nil {
		return err
	}
	var id any
	if userID != nil {
		id = *userID
	}
	if _, err := manager.db.ExecContext(ctx, `
		INSERT INTO webauthn_challenges
			(challenge_hash, user_id, purpose, session_data, remember, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`, tokenDigest(token), id, purpose, sealed, remember, time.Now().Add(passkeyChallengeTTL)); err != nil {
		return err
	}
	setPasskeyChallengeCookie(w, token)
	return nil
}

func (manager *passkeyManager) consumeChallenge(ctx context.Context, r *http.Request, purpose string) (*webauthn.SessionData, *int64, bool, error) {
	cookie, err := r.Cookie("noxio_passkey_challenge")
	if err != nil || len(cookie.Value) != 64 {
		return nil, nil, false, errInvalidToken
	}
	var userID sql.NullInt64
	var encrypted []byte
	var remember bool
	err = manager.db.QueryRowContext(ctx, `
		DELETE FROM webauthn_challenges
		WHERE challenge_hash = $1 AND purpose = $2 AND expires_at > now()
		RETURNING user_id, session_data, COALESCE(remember,false)`, tokenDigest(cookie.Value), purpose).
		Scan(&userID, &encrypted, &remember)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, false, errInvalidToken
	}
	if err != nil {
		return nil, nil, false, err
	}
	plaintext, err := manager.open(encrypted, "webauthn-challenge:"+purpose)
	if err != nil {
		return nil, nil, false, err
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return nil, nil, false, err
	}
	if userID.Valid {
		return &session, &userID.Int64, remember, nil
	}
	return &session, nil, remember, nil
}

func passkeyName(value, userAgent string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "Passkey"
		if strings.Contains(userAgent, "Mac") {
			value = "Mac passkey"
		} else if strings.Contains(userAgent, "iPhone") {
			value = "iPhone passkey"
		} else if strings.Contains(userAgent, "Android") {
			value = "Android passkey"
		}
	}
	runes := []rune(value)
	if len(runes) > 64 {
		value = string(runes[:64])
	}
	return value
}

func registerPasskeys(mux *http.ServeMux, db *sql.DB) {
	manager, managerErr := newPasskeyManager(db)
	if managerErr != nil {
		log.Printf("auth: passkeys disabled: %v", managerErr)
	}

	mux.HandleFunc("GET /api/auth/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"passkeys": manager != nil,
			"oauth":    []string{},
			"password": map[string]int{"minimum": minimumPasswordRunes, "maximum": maximumPasswordRunes},
		})
	})

	mux.HandleFunc("POST /api/auth/passkeys/register/start", func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			writeAuthError(w, http.StatusServiceUnavailable, "passkeys_unavailable")
			return
		}
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "passkey-register", "", ratePolicy{Limit: 10, Window: time.Hour, Block: 10 * time.Minute}) {
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil || !user.Verified {
			writeAuthError(w, http.StatusUnauthorized, "verified_session_required")
			return
		}
		passkeyUser, err := manager.loadUser(r.Context(), user.ID, true)
		if err != nil {
			http.Error(w, "could not start passkey registration", http.StatusInternalServerError)
			return
		}
		creation, session, err := manager.webauthn.BeginRegistration(passkeyUser,
			webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
			webauthn.WithExclusions(webauthn.Credentials(passkeyUser.Credentials).CredentialDescriptors()),
			webauthn.WithExtensions(protocol.AuthenticationExtensions{"credProps": true}),
		)
		if err != nil || manager.saveChallenge(r.Context(), w, &user.ID, "register", session, false) != nil {
			http.Error(w, "could not start passkey registration", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(creation)
	})

	mux.HandleFunc("POST /api/auth/passkeys/register/finish", func(w http.ResponseWriter, r *http.Request) {
		defer clearPasskeyChallengeCookie(w)
		if manager == nil {
			writeAuthError(w, http.StatusServiceUnavailable, "passkeys_unavailable")
			return
		}
		if !requireSameOrigin(w, r) {
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil || !user.Verified {
			writeAuthError(w, http.StatusUnauthorized, "verified_session_required")
			return
		}
		session, challengeUserID, _, err := manager.consumeChallenge(r.Context(), r, "register")
		if err != nil || challengeUserID == nil || *challengeUserID != user.ID {
			writeAuthError(w, http.StatusBadRequest, "invalid_passkey_challenge")
			return
		}
		passkeyUser, err := manager.loadUser(r.Context(), user.ID, false)
		if err != nil {
			http.Error(w, "could not register passkey", http.StatusInternalServerError)
			return
		}
		credential, err := manager.webauthn.FinishRegistration(passkeyUser, *session, r)
		if err != nil {
			writeAuthError(w, http.StatusBadRequest, "passkey_verification_failed")
			return
		}
		encoded, err := json.Marshal(credential)
		if err != nil {
			http.Error(w, "could not register passkey", http.StatusInternalServerError)
			return
		}
		sealed, err := manager.seal(encoded, passkeyCredentialAAD(user.ID, credential.ID))
		if err != nil {
			http.Error(w, "could not register passkey", http.StatusInternalServerError)
			return
		}
		name := passkeyName(r.URL.Query().Get("name"), r.UserAgent())
		if _, err := db.ExecContext(r.Context(), `
			INSERT INTO passkeys (user_id, credential_id, credential_data, name)
			VALUES ($1,$2,$3,$4)`, user.ID, credential.ID, sealed, name); err != nil {
			writeAuthError(w, http.StatusConflict, "passkey_already_registered")
			return
		}
		recordAuthEvent(r.Context(), db, &user.ID, "passkey_registered", r)
		go func() {
			if mailErr := sendPasskeyChangedEmail(user.Email, "added"); mailErr != nil {
				log.Printf("auth: send passkey change notice: %v", mailErr)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("POST /api/auth/passkeys/login/start", func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			writeAuthError(w, http.StatusServiceUnavailable, "passkeys_unavailable")
			return
		}
		if !requireSameOrigin(w, r) || !enforceAuthRateLimit(w, r, "passkey-login", "", ratePolicy{Limit: 20, Window: 10 * time.Minute, Block: 5 * time.Minute}) {
			return
		}
		var req struct {
			Remember bool `json:"remember"`
		}
		if decodeAuthJSON(w, r, &req) != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		assertion, session, err := manager.webauthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
		if err != nil || manager.saveChallenge(r.Context(), w, nil, "login", session, req.Remember) != nil {
			http.Error(w, "could not start passkey login", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(assertion)
	})

	mux.HandleFunc("POST /api/auth/passkeys/login/finish", func(w http.ResponseWriter, r *http.Request) {
		defer clearPasskeyChallengeCookie(w)
		if manager == nil {
			writeAuthError(w, http.StatusServiceUnavailable, "passkeys_unavailable")
			return
		}
		if !requireSameOrigin(w, r) {
			return
		}
		session, _, remember, err := manager.consumeChallenge(r.Context(), r, "login")
		if err != nil {
			writeAuthError(w, http.StatusBadRequest, "invalid_passkey_challenge")
			return
		}
		validatedUser, credential, err := manager.webauthn.FinishPasskeyLogin(func(rawID, userHandle []byte) (webauthn.User, error) {
			return manager.loadUserByHandle(r.Context(), rawID, userHandle)
		}, *session, r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "passkey_verification_failed")
			return
		}
		user, ok := validatedUser.(*passkeyUser)
		if !ok || !user.Verified {
			writeAuthError(w, http.StatusUnauthorized, "passkey_verification_failed")
			return
		}
		encoded, err := json.Marshal(credential)
		if err != nil {
			http.Error(w, "could not update passkey", http.StatusInternalServerError)
			return
		}
		sealed, err := manager.seal(encoded, passkeyCredentialAAD(user.ID, credential.ID))
		if err != nil {
			http.Error(w, "could not update passkey", http.StatusInternalServerError)
			return
		}
		if _, err := db.ExecContext(r.Context(), `
			UPDATE passkeys SET credential_data = $1, last_used_at = now()
			WHERE user_id = $2 AND credential_id = $3`, sealed, user.ID, credential.ID); err != nil {
			http.Error(w, "could not update passkey", http.StatusInternalServerError)
			return
		}
		hadSessions, knownDevice := knownLoginDevice(r.Context(), db, user.ID, r.UserAgent())
		if err := issueUserSession(w, r, db, user.ID, sessionOptions{Remember: remember, AuthMethod: "passkey"}); err != nil {
			http.Error(w, "could not create session", http.StatusInternalServerError)
			return
		}
		recordAuthEvent(r.Context(), db, &user.ID, "passkey_login_succeeded", r)
		if hadSessions && !knownDevice {
			go func() {
				if mailErr := sendNewLoginEmail(user.Email, safeUserAgent(r.UserAgent()), requestIPHint(r)); mailErr != nil {
					log.Printf("auth: send new passkey login notice: %v", mailErr)
				}
			}()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("GET /api/auth/passkeys", func(w http.ResponseWriter, r *http.Request) {
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		rows, err := db.QueryContext(r.Context(), `
			SELECT id, name, created_at, last_used_at FROM passkeys WHERE user_id = $1 ORDER BY created_at DESC`, user.ID)
		if err != nil {
			http.Error(w, "could not list passkeys", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		type passkeyView struct {
			ID         int64      `json:"id"`
			Name       string     `json:"name"`
			CreatedAt  time.Time  `json:"created_at"`
			LastUsedAt *time.Time `json:"last_used_at,omitempty"`
		}
		items := make([]passkeyView, 0, 2)
		for rows.Next() {
			var item passkeyView
			var lastUsed sql.NullTime
			if err := rows.Scan(&item.ID, &item.Name, &item.CreatedAt, &lastUsed); err != nil {
				http.Error(w, "could not list passkeys", http.StatusInternalServerError)
				return
			}
			if lastUsed.Valid {
				item.LastUsedAt = &lastUsed.Time
			}
			items = append(items, item)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"passkeys": items})
	})

	mux.HandleFunc("DELETE /api/auth/passkeys/{passkeyID}", func(w http.ResponseWriter, r *http.Request) {
		if !requireSameOrigin(w, r) {
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil || user == nil {
			writeAuthError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		passkeyID, err := strconv.ParseInt(r.PathValue("passkeyID"), 10, 64)
		if err != nil || passkeyID <= 0 {
			writeAuthError(w, http.StatusBadRequest, "invalid_passkey")
			return
		}
		result, err := db.ExecContext(r.Context(), `DELETE FROM passkeys WHERE id = $1 AND user_id = $2`, passkeyID, user.ID)
		if err != nil {
			http.Error(w, "could not delete passkey", http.StatusInternalServerError)
			return
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			recordAuthEvent(r.Context(), db, &user.ID, "passkey_deleted", r)
			go func() {
				if mailErr := sendPasskeyChangedEmail(user.Email, "removed"); mailErr != nil {
					log.Printf("auth: send passkey change notice: %v", mailErr)
				}
			}()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
}
