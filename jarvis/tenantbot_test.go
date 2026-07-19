package main

import (
	"encoding/hex"
	"testing"
)

func TestWebhookSecretMatches(t *testing.T) {
	t.Parallel()

	secret := "b79bc940e7a36c4a32813e183227713fd5ddf337f32a9b9467079d917cc025db"
	tests := []struct {
		name   string
		header string
		path   string
		want   bool
	}{
		{name: "matching secrets", header: secret, path: secret, want: true},
		{name: "different same-length secret", header: secret[:63] + "0", path: secret},
		{name: "different length", header: secret[:32], path: secret},
		{name: "empty secrets", header: "", path: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := webhookSecretMatches(tt.header, tt.path); got != tt.want {
				t.Fatalf("webhookSecretMatches() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestNewTenantWebhookSecretUses32RandomBytes(t *testing.T) {
	t.Parallel()

	secret, err := newTenantWebhookSecret()
	if err != nil {
		t.Fatalf("newTenantWebhookSecret() error = %v", err)
	}
	decoded, err := hex.DecodeString(secret)
	if err != nil {
		t.Fatalf("webhook secret is not hexadecimal: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("webhook secret contains %d bytes; want 32", len(decoded))
	}
}

func TestParseTenantTelegramUpdate(t *testing.T) {
	t.Parallel()

	updateJSON := []byte(`{
		"update_id": 741,
		"message": {
			"message_id": 29,
			"from": {
				"id": 90901,
				"is_bot": false,
				"first_name": "Mina",
				"last_name": "Karimi",
				"username": "mina_k"
			},
			"chat": {
				"id": -1001234567890,
				"type": "supergroup",
				"title": "Customers"
			},
			"text": "Do you deliver on Fridays?"
		}
	}`)

	got, err := parseTenantTelegramUpdate(updateJSON)
	if err != nil {
		t.Fatalf("parseTenantTelegramUpdate() error = %v", err)
	}
	if got.ChatID != -1001234567890 {
		t.Errorf("ChatID = %d; want -1001234567890", got.ChatID)
	}
	if got.Text != "Do you deliver on Fridays?" {
		t.Errorf("Text = %q; want %q", got.Text, "Do you deliver on Fridays?")
	}
	if got.FromName != "Mina Karimi" {
		t.Errorf("FromName = %q; want %q", got.FromName, "Mina Karimi")
	}
}

func TestParseTenantTelegramUpdateRejectsMissingText(t *testing.T) {
	t.Parallel()

	for _, input := range [][]byte{
		[]byte(`{"update_id": 1}`),
		[]byte(`{"update_id": 2, "message": {"chat": {"id": 42}, "text": "  "}}`),
		[]byte(`not-json`),
	} {
		if _, err := parseTenantTelegramUpdate(input); err == nil {
			t.Fatalf("parseTenantTelegramUpdate(%q) expected an error", input)
		}
	}
}

func TestTenantEscalationDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{name: "customer asks for human", got: shouldEscalate("Please let me talk to a human"), want: true},
		{name: "customer asks in Persian", got: shouldEscalate("لطفاً نماینده پاسخ بدهد"), want: true},
		{name: "ordinary customer question", got: shouldEscalate("What time do you open?"), want: false},
		{name: "model cannot answer", got: brainNeedsEscalation("I don't know from the available information."), want: true},
		{name: "grounded model answer", got: brainNeedsEscalation("We open at 9 AM."), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Fatalf("escalation = %v; want %v", tt.got, tt.want)
			}
		})
	}
}
