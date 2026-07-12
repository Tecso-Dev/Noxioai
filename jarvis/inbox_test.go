package main

import "testing"

func TestNormalizeInboxEmail(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want string
	}{
		{"  JANE.DOE@Example.COM  ", "jane.doe@example.com"},
		{"", ""},
		{"  ", ""},
	} {
		if got := normalizeInboxEmail(tt.in); got != tt.want {
			t.Errorf("normalizeInboxEmail(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
