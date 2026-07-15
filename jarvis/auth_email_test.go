package main

import (
	"database/sql"
	"testing"
	"time"
)

func TestGenerateAuthTokenUniqueAndLength(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		tok, err := generateAuthToken()
		if err != nil {
			t.Fatalf("generateAuthToken: %v", err)
		}
		if len(tok) != 64 { // 32 random bytes, hex-encoded
			t.Fatalf("token length = %d, want 64 (got %q)", len(tok), tok)
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %s", tok)
		}
		seen[tok] = true
	}
}

func TestTokenValid(t *testing.T) {
	now := time.Now()
	notUsed := sql.NullTime{}
	used := sql.NullTime{Time: now.Add(-time.Minute), Valid: true}

	cases := []struct {
		name    string
		usedAt  sql.NullTime
		expires time.Time
		want    bool
	}{
		{"fresh and unused", notUsed, now.Add(time.Hour), true},
		{"expired", notUsed, now.Add(-time.Hour), false},
		{"already used", used, now.Add(time.Hour), false},
		{"used and expired", used, now.Add(-time.Hour), false},
	}
	for _, c := range cases {
		if got := tokenValid(c.usedAt, c.expires, now); got != c.want {
			t.Errorf("%s: tokenValid() = %v, want %v", c.name, got, c.want)
		}
	}
}
