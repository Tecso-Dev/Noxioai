package main

import (
	"bufio"
	"context"
	"crypto/sha1" // Pwned Passwords specifies SHA-1 prefixes; password storage remains Argon2id.
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	minimumPasswordRunes = 15
	maximumPasswordRunes = 256
	defaultSessionTTL    = 12 * time.Hour
	rememberedSessionTTL = 30 * 24 * time.Hour
	maximumAuthBodyBytes = 16 * 1024
)

var (
	errPasswordTooShort    = errors.New("password_too_short")
	errPasswordTooLong     = errors.New("password_too_long")
	errPasswordCompromised = errors.New("password_compromised")
	errPasswordScreening   = errors.New("password_screening_unavailable")
	errInvalidEmail        = errors.New("invalid_email")
	errInvalidUsername     = errors.New("invalid_username")

	usernamePattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)
	pwnedPasswordsEndpoint = "https://api.pwnedpasswords.com/range/"
	pwnedPasswordsClient   = &http.Client{Timeout: 3 * time.Second}
)

var commonPasswords = map[string]struct{}{
	"123456789012345":              {},
	"correct horse battery staple": {},
	"iloveyouiloveyou":             {},
	"letmeinletmein123":            {},
	"passwordpassword":             {},
	"qwertyuiopasdfgh":             {},
	"thisisapassword":              {},
	"welcome123456789":             {},
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" || len(email) > 254 || strings.Count(email, "@") != 1 {
		return "", errInvalidEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", errInvalidEmail
	}
	return email, nil
}

func normalizeUsername(value string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(value))
	if !usernamePattern.MatchString(username) {
		return "", errInvalidUsername
	}
	return username, nil
}

func normalizeDisplayName(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 100 {
		value = string(runes[:100])
	}
	return value
}

func validatePasswordLocally(password string) error {
	password = norm.NFC.String(password)
	length := utf8.RuneCountInString(password)
	if length < minimumPasswordRunes {
		return errPasswordTooShort
	}
	if length > maximumPasswordRunes {
		return errPasswordTooLong
	}
	if _, found := commonPasswords[strings.ToLower(password)]; found {
		return errPasswordCompromised
	}
	return nil
}

func validatePassword(ctx context.Context, password string) error {
	if err := validatePasswordLocally(password); err != nil {
		return err
	}
	compromised, err := passwordHasBeenPwned(ctx, password)
	if err != nil {
		// Password establishment fails closed because the product policy requires
		// breached-password blocking. Existing-account login never calls this service.
		log.Printf("auth: breached-password screening unavailable: %v", err)
		return errPasswordScreening
	}
	if compromised {
		return errPasswordCompromised
	}
	return nil
}

func passwordHasBeenPwned(ctx context.Context, password string) (bool, error) {
	password = norm.NFC.String(password)
	digest := sha1.Sum([]byte(password))
	hash := strings.ToUpper(hex.EncodeToString(digest[:]))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pwnedPasswordsEndpoint+hash[:5], nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Add-Padding", "true")
	req.Header.Set("User-Agent", "NOXIOAI-password-screening")
	resp, err := pwnedPasswordsClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("pwned passwords: %s", resp.Status)
	}
	suffix := hash[5:]
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 2<<20))
	for scanner.Scan() {
		candidate, _, found := strings.Cut(strings.TrimSpace(scanner.Text()), ":")
		if found && strings.EqualFold(candidate, suffix) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maximumAuthBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func tokenDigest(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func shortHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:16])
}

func requestIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	ip := net.ParseIP(host)
	// The API listens on loopback behind Caddy. Caddy overwrites X-Real-IP;
	// never trust this header from a directly connected non-loopback client.
	if ip != nil && ip.IsLoopback() {
		if forwarded := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); forwarded != nil {
			return forwarded
		}
	}
	return ip
}

func requestIPHint(r *http.Request) string {
	ip := requestIP(r)
	if ip == nil {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.0/24", ipv4[0], ipv4[1], ipv4[2])
	}
	masked := ip.Mask(net.CIDRMask(48, 128))
	return masked.String() + "/48"
}

func safeUserAgent(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	runes := []rune(value)
	if len(runes) > 180 {
		value = string(runes[:180])
	}
	return value
}

type ratePolicy struct {
	Limit  int
	Window time.Duration
	Block  time.Duration
}

type rateEntry struct {
	windowStarted time.Time
	lastSeen      time.Time
	blockedUntil  time.Time
	count         int
	strikes       int
}

type authRateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateEntry
	failures map[string]*rateEntry
	now      func() time.Time
}

func newAuthRateLimiter() *authRateLimiter {
	return &authRateLimiter{
		entries:  make(map[string]*rateEntry),
		failures: make(map[string]*rateEntry),
		now:      time.Now,
	}
}

func (limiter *authRateLimiter) allow(key string, policy ratePolicy) (bool, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	entry := limiter.entries[key]
	if entry == nil {
		entry = &rateEntry{windowStarted: now}
		limiter.entries[key] = entry
	}
	entry.lastSeen = now
	if now.Before(entry.blockedUntil) {
		return false, entry.blockedUntil.Sub(now)
	}
	if now.Sub(entry.windowStarted) >= policy.Window {
		entry.windowStarted = now
		entry.count = 0
		if entry.strikes > 0 {
			entry.strikes--
		}
	}
	entry.count++
	if entry.count <= policy.Limit {
		return true, 0
	}
	entry.strikes++
	shift := min(entry.strikes-1, 4)
	penalty := policy.Block * time.Duration(1<<shift)
	entry.blockedUntil = now.Add(penalty)
	return false, penalty
}

func (limiter *authRateLimiter) recordFailure(key string) time.Duration {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	entry := limiter.failures[key]
	if entry == nil || now.Sub(entry.lastSeen) > 30*time.Minute {
		entry = &rateEntry{}
		limiter.failures[key] = entry
	}
	entry.lastSeen = now
	entry.count++
	return failedLoginDelay(entry.count)
}

func (limiter *authRateLimiter) clearFailures(key string) {
	limiter.mu.Lock()
	delete(limiter.failures, key)
	limiter.mu.Unlock()
}

func failedLoginDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	shift := min(attempt-1, 4)
	return 250 * time.Millisecond * time.Duration(1<<shift)
}

var authLimiter = newAuthRateLimiter()

func authRateKey(scope string, r *http.Request, identifier string) string {
	ip := "unknown"
	if parsed := requestIP(r); parsed != nil {
		ip = parsed.String()
	}
	return shortHash(scope + "|" + ip + "|" + strings.ToLower(strings.TrimSpace(identifier)))
}

func enforceAuthRateLimit(w http.ResponseWriter, r *http.Request, scope, identifier string, policy ratePolicy) bool {
	allowed, retryAfter := authLimiter.allow(authRateKey(scope, r, identifier), policy)
	if allowed {
		return true
	}
	seconds := max(1, int(retryAfter.Round(time.Second).Seconds()))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeAuthError(w, http.StatusTooManyRequests, "too_many_attempts")
	return false
}

func allowedAuthOrigin(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	allowed := make(map[string]bool)
	if base, err := url.Parse(appBaseURL()); err == nil && base.Scheme != "" && base.Host != "" {
		allowed[base.Scheme+"://"+base.Host] = true
	}
	for _, configured := range strings.Split(os.Getenv("AUTH_ALLOWED_ORIGINS"), ",") {
		if configured = strings.TrimSpace(configured); configured != "" {
			allowed[strings.TrimSuffix(configured, "/")] = true
		}
	}
	if strings.HasPrefix(r.Host, "localhost:") || strings.HasPrefix(r.Host, "127.0.0.1:") {
		allowed["http://"+r.Host] = true
	}
	return allowed[strings.TrimSuffix(origin, "/")]
}

func requireSameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if allowedAuthOrigin(r) {
		return true
	}
	writeAuthError(w, http.StatusForbidden, "forbidden_origin")
	return false
}

func recordAuthEvent(ctx context.Context, db *sql.DB, userID *int64, event string, r *http.Request) {
	if db == nil {
		return
	}
	var id any
	if userID != nil {
		id = *userID
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO auth_audit_log (user_id, event, ip_hint, user_agent)
		VALUES ($1,$2,$3,$4)`, id, event, requestIPHint(r), safeUserAgent(r.UserAgent())); err != nil {
		log.Printf("auth: record audit event %s: %v", event, err)
	}
}

func authSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if strings.HasPrefix(r.URL.Path, "/api/auth/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		// Cookie-authenticated state changes must come from the application
		// origin. SameSite remains defense in depth; this explicit check covers
		// browsers and embedded contexts that do send cookies.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if _, err := r.Cookie("jarvis_session"); err == nil {
				if strings.TrimSpace(r.Header.Get("Origin")) == "" || !allowedAuthOrigin(r) {
					writeAuthError(w, http.StatusForbidden, "forbidden_origin")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
