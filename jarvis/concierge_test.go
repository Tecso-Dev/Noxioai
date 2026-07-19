package main

import (
	"strings"
	"testing"
)

func TestTrimConciergeHistoryCapsAndSanitizes(t *testing.T) {
	history := make([]Message, 0, 12)
	for i := 0; i < 12; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, Message{Role: role, Content: string(rune('a' + i))})
	}
	history[5].Role = "system"

	got := trimConciergeHistory(history)
	if len(got) != 7 {
		t.Fatalf("trimConciergeHistory() length = %d, want 7 valid messages from the last 8", len(got))
	}
	if got[0].Content != "e" || got[len(got)-1].Content != "l" {
		t.Fatalf("trimConciergeHistory() kept %q through %q, want e through l", got[0].Content, got[len(got)-1].Content)
	}
	for _, message := range got {
		if message.Role != "user" && message.Role != "assistant" {
			t.Fatalf("trimConciergeHistory() retained unsafe role %q", message.Role)
		}
	}
}

func TestNormalizeConciergeMessageLengthGuard(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{name: "trims", message: "  چطور ربات بسازم؟  ", wantErr: false},
		{name: "empty", message: " \n\t ", wantErr: true},
		{name: "at limit", message: strings.Repeat("م", conciergeMessageMaxRunes), wantErr: false},
		{name: "over limit", message: strings.Repeat("م", conciergeMessageMaxRunes+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeConciergeMessage(tt.message)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeConciergeMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.name == "trims" && got != "چطور ربات بسازم؟" {
				t.Fatalf("normalizeConciergeMessage() = %q, want trimmed message", got)
			}
		})
	}
}

func TestConciergeSystemPromptContainsGroundingAndBotFatherSteps(t *testing.T) {
	prompt := conciergeSystemPrompt("کافه سپید", "ساعت کاری: ۸ تا ۲۰")
	wants := []string{
		"You are the NOXIOAI setup concierge",
		"Only answer questions about using NOXIOAI",
		"politely redirect",
		"The only live agent today is the Telegram Customer-Response agent",
		"Every other agent is coming soon",
		"Open Telegram",
		"Open @BotFather",
		"Send /newbot",
		"ending in \"bot\"",
		"Connect your Telegram bot",
		"کافه سپید",
		"ساعت کاری: ۸ تا ۲۰",
	}
	for _, want := range wants {
		if !strings.Contains(prompt, want) {
			t.Errorf("conciergeSystemPrompt() missing %q", want)
		}
	}
}
