package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPasswordPolicySupportsLongUnicodePassphrases(t *testing.T) {
	if err := validatePasswordLocally("pąss phrase with spaces ✨"); err != nil {
		t.Fatalf("valid Unicode passphrase rejected: %v", err)
	}
	if err := validatePasswordLocally("too short"); err != errPasswordTooShort {
		t.Fatalf("short password error = %v, want %v", err, errPasswordTooShort)
	}
	if err := validatePasswordLocally("passwordpassword"); err != errPasswordCompromised {
		t.Fatalf("common password error = %v, want %v", err, errPasswordCompromised)
	}
}

func TestIdentityNormalization(t *testing.T) {
	email, err := normalizeEmail("  Person@Example.COM ")
	if err != nil || email != "person@example.com" {
		t.Fatalf("normalizeEmail = %q, %v", email, err)
	}
	username, err := normalizeUsername("  Noxio.User ")
	if err != nil || username != "noxio.user" {
		t.Fatalf("normalizeUsername = %q, %v", username, err)
	}
	for _, invalid := range []string{"ab", "has space", "@admin", strings.Repeat("a", 33)} {
		if _, err := normalizeUsername(invalid); err != errInvalidUsername {
			t.Fatalf("normalizeUsername(%q) error = %v", invalid, err)
		}
	}
}

func TestPwnedPasswordCheckUsesOnlyHashPrefix(t *testing.T) {
	password := "unique testing passphrase 2026"
	digest := sha1.Sum([]byte(password))
	hash := strings.ToUpper(hex.EncodeToString(digest[:]))
	var requestedPath, sentBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		sentBody = string(body)
		if r.Header.Get("Add-Padding") != "true" {
			t.Error("Add-Padding header was not sent")
		}
		_, _ = w.Write([]byte(hash[5:] + ":42\nABCDEF:0\n"))
	}))
	defer server.Close()

	originalEndpoint, originalClient := pwnedPasswordsEndpoint, pwnedPasswordsClient
	pwnedPasswordsEndpoint = server.URL + "/range/"
	pwnedPasswordsClient = server.Client()
	defer func() {
		pwnedPasswordsEndpoint = originalEndpoint
		pwnedPasswordsClient = originalClient
	}()

	compromised, err := passwordHasBeenPwned(context.Background(), password)
	if err != nil || !compromised {
		t.Fatalf("passwordHasBeenPwned = %v, %v", compromised, err)
	}
	if requestedPath != "/range/"+hash[:5] {
		t.Fatalf("requested path = %q, want only the SHA-1 prefix", requestedPath)
	}
	if sentBody != "" || strings.Contains(requestedPath, password) || strings.Contains(requestedPath, hash[5:]) {
		t.Fatal("password or full hash leaked to the screening service")
	}
}

func TestPasswordEstablishmentFailsClosedWhenScreeningIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	originalEndpoint, originalClient := pwnedPasswordsEndpoint, pwnedPasswordsClient
	pwnedPasswordsEndpoint = server.URL + "/range/"
	pwnedPasswordsClient = server.Client()
	defer func() {
		pwnedPasswordsEndpoint = originalEndpoint
		pwnedPasswordsClient = originalClient
	}()

	if err := validatePassword(context.Background(), "a unique private passphrase"); err != errPasswordScreening {
		t.Fatalf("validatePassword error = %v, want %v", err, errPasswordScreening)
	}
}

func TestRateLimiterTemporarilyThrottlesAndRecovers(t *testing.T) {
	limiter := newAuthRateLimiter()
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	policy := ratePolicy{Limit: 2, Window: time.Minute, Block: 10 * time.Second}
	if ok, _ := limiter.allow("login", policy); !ok {
		t.Fatal("first attempt blocked")
	}
	if ok, _ := limiter.allow("login", policy); !ok {
		t.Fatal("second attempt blocked")
	}
	if ok, retry := limiter.allow("login", policy); ok || retry != 10*time.Second {
		t.Fatalf("third attempt = ok %v retry %v", ok, retry)
	}
	now = now.Add(11 * time.Second)
	if ok, _ := limiter.allow("login", policy); ok {
		t.Fatal("attempt should remain over the current window limit after the temporary block")
	}
	now = now.Add(time.Minute)
	if ok, _ := limiter.allow("login", policy); !ok {
		t.Fatal("limiter did not recover after its window")
	}
}

func TestFailedLoginDelayIsProgressiveAndBounded(t *testing.T) {
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}
	for index, expected := range want {
		if got := failedLoginDelay(index + 1); got != expected {
			t.Fatalf("attempt %d delay = %v, want %v", index+1, got, expected)
		}
	}
}

func TestOriginAndForwardedIPValidation(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://noxioai.com")
	req := httptest.NewRequest(http.MethodPost, "https://api.noxioai.com/api/auth/login", nil)
	req.Header.Set("Origin", "https://evil.example")
	if allowedAuthOrigin(req) {
		t.Fatal("cross-origin authentication request was accepted")
	}
	req.Header.Set("Origin", "https://noxioai.com")
	if !allowedAuthOrigin(req) {
		t.Fatal("configured application origin was rejected")
	}

	direct := httptest.NewRequest(http.MethodPost, "/", nil)
	direct.RemoteAddr = "203.0.113.5:443"
	direct.Header.Set("X-Real-IP", "198.51.100.10")
	if got := requestIP(direct).String(); got != "203.0.113.5" {
		t.Fatalf("untrusted forwarded IP was accepted: %s", got)
	}
	proxied := httptest.NewRequest(http.MethodPost, "/", nil)
	proxied.RemoteAddr = "127.0.0.1:12345"
	proxied.Header.Set("X-Real-IP", "198.51.100.10")
	if got := requestIP(proxied).String(); got != "198.51.100.10" {
		t.Fatalf("trusted reverse-proxy IP was ignored: %s", got)
	}
}

func TestPasskeyEncryptionRequiresKeyAndAuthenticatesContext(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://noxioai.com")
	t.Setenv("JARVIS_AUTH_DATA_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	manager, err := newPasskeyManager(&sql.DB{})
	if err != nil {
		t.Fatalf("newPasskeyManager: %v", err)
	}
	ciphertext, err := manager.seal([]byte("credential material"), "passkey:7:abc")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	plaintext, err := manager.open(ciphertext, "passkey:7:abc")
	if err != nil || string(plaintext) != "credential material" {
		t.Fatalf("open = %q, %v", plaintext, err)
	}
	if _, err := manager.open(ciphertext, "passkey:8:abc"); err == nil {
		t.Fatal("encrypted passkey opened under a different account context")
	}
}

func TestAuthSecurityHeaders(t *testing.T) {
	handler := authSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	for name, expected := range map[string]string{
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
	} {
		if got := response.Header().Get(name); got != expected {
			t.Errorf("%s = %q, want %q", name, got, expected)
		}
	}
}

func TestAuthSecurityHeadersRejectCookieMutationWithoutOrigin(t *testing.T) {
	t.Setenv("APP_BASE_URL", "https://noxioai.com")
	handler := authSecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("cross-site request reached the application handler")
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/profile", nil)
	request.AddCookie(&http.Cookie{Name: "jarvis_session", Value: "secret"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}
