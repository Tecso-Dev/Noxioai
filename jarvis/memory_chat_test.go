package main

import (
	"strings"
	"testing"
	"time"
)

// TestChatSystemPromptOrder mirrors registerChat's assembly: base prompt,
// then owner memory (profile + facts), then the CRM snapshot — in that
// order, each clearly sectioned, so the admin console never loses either
// personal context or live business data.
func TestChatSystemPromptOrder(t *testing.T) {
	t.Parallel()
	memory := &MemoryStore{
		Profile: "Sobhan runs NOXIOAI and prefers concise answers.",
		Facts:   []Fact{{Text: "Prefers Persian in the evenings.", Learned: time.Now(), Source: "auto"}},
	}
	crmSnapshot := "3 leads, 1 outreach pending."

	system := systemPrompt + memory.SystemContext() +
		"\n\n## Live business data (real, current — answer from THIS, never invent)\n" + crmSnapshot

	iBase := strings.Index(system, "You are JARVIS")
	iProfile := strings.Index(system, "Sobhan runs NOXIOAI")
	iFacts := strings.Index(system, "Prefers Persian in the evenings.")
	iCRM := strings.Index(system, "3 leads, 1 outreach pending.")

	if iBase == -1 || iProfile == -1 || iFacts == -1 || iCRM == -1 {
		t.Fatalf("expected all four sections present, got:\n%s", system)
	}
	if !(iBase < iProfile && iProfile < iFacts && iFacts < iCRM) {
		t.Fatalf("expected order base < profile < facts < CRM, got indices %d %d %d %d", iBase, iProfile, iFacts, iCRM)
	}
}

// TestShapeChatHistoryOrdersOldestFirst checks the pure JSON-shaping helper
// used by GET /api/chat/history: DB rows arrive newest-first (ORDER BY
// created_at DESC), the API response must read oldest-first.
func TestShapeChatHistoryOrdersOldestFirst(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rows := []chatRow{
		{Role: "assistant", Content: "third", CreatedAt: now},
		{Role: "user", Content: "second", CreatedAt: now.Add(-time.Minute)},
		{Role: "user", Content: "first", CreatedAt: now.Add(-2 * time.Minute)},
	}

	turns := shapeChatHistory(rows)

	if len(turns) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(turns))
	}
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if turns[i].Content != w {
			t.Fatalf("turn %d: want %q, got %q", i, w, turns[i].Content)
		}
	}
	if turns[0].Role != "user" || turns[2].Role != "assistant" {
		t.Fatalf("roles not preserved: %+v", turns)
	}
	if turns[0].Ts == "" {
		t.Fatal("expected non-empty timestamp")
	}
}

// TestShapeChatHistoryEmpty guards against a nil slice serializing as JSON
// null instead of [] for the console's boot-load fetch.
func TestShapeChatHistoryEmpty(t *testing.T) {
	t.Parallel()
	turns := shapeChatHistory(nil)
	if turns == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(turns) != 0 {
		t.Fatalf("expected 0 turns, got %d", len(turns))
	}
}
