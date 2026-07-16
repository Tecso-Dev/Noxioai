package main

import "testing"

func TestIsValidWaitlistEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "standard address", email: "person@example.com", want: true},
		{name: "address with tag", email: "person+early@example.co.uk", want: true},
		{name: "surrounding whitespace", email: "  person@example.com  ", want: true},
		{name: "empty", email: "", want: false},
		{name: "missing at sign", email: "person.example.com", want: false},
		{name: "missing local part", email: "@example.com", want: false},
		{name: "missing domain", email: "person@", want: false},
		{name: "display name", email: "Person <person@example.com>", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidWaitlistEmail(tt.email); got != tt.want {
				t.Errorf("isValidWaitlistEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}
