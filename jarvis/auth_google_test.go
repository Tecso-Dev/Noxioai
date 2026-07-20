package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func makeTestIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(header) + "." + enc.EncodeToString(payload) + "." + enc.EncodeToString([]byte("signature"))
}

func TestGoogleStateCookieRoundTrip(t *testing.T) {
	t.Parallel()

	state, err := generateAuthToken()
	if err != nil {
		t.Fatalf("generateAuthToken: %v", err)
	}

	rec := httptest.NewRecorder()
	setGoogleStateCookie(rec, state)

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == googleStateCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("state cookie was not set")
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("state cookie missing security flags: %+v", cookie)
	}
	if cookie.Path != googleStatePath {
		t.Fatalf("state cookie path = %q, want %q", cookie.Path, googleStatePath)
	}
	if !verifyState(cookie.Value, state) {
		t.Fatal("verifyState rejected the round-tripped state")
	}
	if verifyState(cookie.Value, "not-the-state-value") {
		t.Fatal("verifyState accepted a mismatched state")
	}
	if verifyState("", "") {
		t.Fatal("verifyState accepted two empty values")
	}

	clearRec := httptest.NewRecorder()
	clearGoogleStateCookie(clearRec)
	cleared := clearRec.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatalf("clearGoogleStateCookie did not expire the cookie: %+v", cleared)
	}
}

func TestParseGoogleIDTokenRejectsGarbage(t *testing.T) {
	t.Parallel()

	for _, token := range []string{
		"",
		"not-a-jwt",
		"only.two",
		"a.b.c.d",
		"a." + "***not-base64url***" + ".c",
	} {
		if _, err := parseGoogleIDToken(token); err == nil {
			t.Fatalf("parseGoogleIDToken(%q): expected error, got none", token)
		}
	}
}

func TestValidateGoogleClaims(t *testing.T) {
	t.Parallel()

	const clientID = "client-under-test.apps.googleusercontent.com"
	now := time.Now()
	baseClaims := func() map[string]any {
		return map[string]any{
			"iss":            "https://accounts.google.com",
			"aud":            clientID,
			"sub":            "108000000000000000001",
			"email":          "person@example.com",
			"email_verified": true,
			"exp":            now.Add(time.Hour).Unix(),
		}
	}

	cases := []struct {
		name    string
		mutate  func(map[string]any)
		wantErr bool
	}{
		{name: "valid token accepted", mutate: func(map[string]any) {}, wantErr: false},
		{name: "expired token rejected", mutate: func(c map[string]any) { c["exp"] = now.Add(-time.Hour).Unix() }, wantErr: true},
		{name: "wrong audience rejected", mutate: func(c map[string]any) { c["aud"] = "someone-elses-client-id" }, wantErr: true},
		{name: "unverified email rejected", mutate: func(c map[string]any) { c["email_verified"] = false }, wantErr: true},
		{name: "string false email_verified rejected", mutate: func(c map[string]any) { c["email_verified"] = "false" }, wantErr: true},
		{name: "wrong issuer rejected", mutate: func(c map[string]any) { c["iss"] = "https://not-google.example" }, wantErr: true},
		{name: "bare accounts.google.com issuer accepted", mutate: func(c map[string]any) { c["iss"] = "accounts.google.com" }, wantErr: false},
		{name: "missing sub rejected", mutate: func(c map[string]any) { c["sub"] = "" }, wantErr: true},
		{name: "missing email rejected", mutate: func(c map[string]any) { c["email"] = "" }, wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			claims := baseClaims()
			tc.mutate(claims)
			token := makeTestIDToken(t, claims)

			parsed, err := parseGoogleIDToken(token)
			if err != nil {
				t.Fatalf("parseGoogleIDToken: %v", err)
			}
			err = validateGoogleClaims(parsed, clientID, now)
			if tc.wantErr && err == nil {
				t.Fatalf("validateGoogleClaims: expected an error, got none")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateGoogleClaims: unexpected error: %v", err)
			}
		})
	}

	if err := validateGoogleClaims(nil, clientID, now); err == nil {
		t.Fatal("validateGoogleClaims(nil, ...) should reject a nil claims pointer")
	}
	if err := validateGoogleClaims(&googleIDTokenClaims{}, "", now); err == nil {
		t.Fatal("validateGoogleClaims with an empty configured clientID should reject")
	}
}

func TestDecideGoogleAccountAction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                     string
		subMatched, emailMatched bool
		want                     googleAccountAction
	}{
		{name: "sub match logs in", subMatched: true, emailMatched: false, want: googleActionLogin},
		{name: "sub match wins even with an email match", subMatched: true, emailMatched: true, want: googleActionLogin},
		{name: "email-only match links the account", subMatched: false, emailMatched: true, want: googleActionLink},
		{name: "no match creates an account", subMatched: false, emailMatched: false, want: googleActionSignup},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := decideGoogleAccountAction(tc.subMatched, tc.emailMatched)
			if got != tc.want {
				t.Fatalf("decideGoogleAccountAction(%v, %v) = %v, want %v", tc.subMatched, tc.emailMatched, got, tc.want)
			}
		})
	}
}

func TestGoogleAccountActionEvent(t *testing.T) {
	t.Parallel()

	cases := map[googleAccountAction]string{
		googleActionLogin:  "auth.google.login",
		googleActionLink:   "auth.google.link",
		googleActionSignup: "auth.google.signup",
	}
	for action, want := range cases {
		if got := googleAccountActionEvent(action); got != want {
			t.Fatalf("googleAccountActionEvent(%v) = %q, want %q", action, got, want)
		}
	}
}
