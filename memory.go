package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryStore is how JARVIS learns Sobhan over time.
// profile.md   — stable identity, always injected into the system prompt.
// facts.jsonl  — one learned fact per line, appended as conversations happen.
// Everything stays local. This is personal data — never synced, never committed.
type MemoryStore struct {
	Dir     string
	Profile string
	Facts   []Fact
}

type Fact struct {
	Text    string    `json:"text"`
	Learned time.Time `json:"learned"`
	Source  string    `json:"source"` // "auto" (extracted) or "explicit" (/remember)
}

func LoadMemory() *MemoryStore {
	dir := envOr("JARVIS_MEMORY_DIR", filepath.Join(os.Getenv("HOME"), "Documents", "jarvis", "memory"))
	os.MkdirAll(dir, 0o755)
	m := &MemoryStore{Dir: dir}

	if b, err := os.ReadFile(filepath.Join(dir, "profile.md")); err == nil {
		m.Profile = string(b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "facts.jsonl")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var f Fact
			if json.Unmarshal([]byte(line), &f) == nil {
				m.Facts = append(m.Facts, f)
			}
		}
	}
	return m
}

// SystemContext renders what JARVIS knows for injection into the system prompt.
func (m *MemoryStore) SystemContext() string {
	var b strings.Builder
	if m.Profile != "" {
		b.WriteString("\n\n## What you know about Sobhan (profile)\n")
		b.WriteString(m.Profile)
	}
	if len(m.Facts) > 0 {
		b.WriteString("\n\n## Facts learned from past conversations\n")
		// newest facts are most relevant; cap injection to the last 100
		start := 0
		if len(m.Facts) > 100 {
			start = len(m.Facts) - 100
		}
		for _, f := range m.Facts[start:] {
			b.WriteString("- " + f.Text + "\n")
		}
	}
	b.WriteString("\nUse this knowledge naturally. Never recite it unprompted; never claim to know things not listed.")
	return b.String()
}

// Remember adds an explicit fact ("/remember ..." or "remember that ...").
func (m *MemoryStore) Remember(text, source string) error {
	text = strings.TrimSpace(text)
	if text == "" || m.isDuplicate(text) {
		return nil
	}
	f := Fact{Text: text, Learned: time.Now(), Source: source}
	m.Facts = append(m.Facts, f)
	line, _ := json.Marshal(f)
	fh, err := os.OpenFile(filepath.Join(m.Dir, "facts.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = fh.Write(append(line, '\n'))
	return err
}

// Forget removes facts containing the given text (case-insensitive) and rewrites the journal.
func (m *MemoryStore) Forget(substr string) int {
	substr = strings.ToLower(strings.TrimSpace(substr))
	if substr == "" {
		return 0
	}
	var kept []Fact
	removed := 0
	for _, f := range m.Facts {
		if strings.Contains(strings.ToLower(f.Text), substr) {
			removed++
		} else {
			kept = append(kept, f)
		}
	}
	if removed > 0 {
		m.Facts = kept
		var b strings.Builder
		for _, f := range kept {
			line, _ := json.Marshal(f)
			b.Write(line)
			b.WriteByte('\n')
		}
		os.WriteFile(filepath.Join(m.Dir, "facts.jsonl"), []byte(b.String()), 0o644)
	}
	return removed
}

func (m *MemoryStore) isDuplicate(text string) bool {
	t := strings.ToLower(text)
	for _, f := range m.Facts {
		if strings.ToLower(f.Text) == t {
			return true
		}
	}
	return false
}

const extractPrompt = `You extract durable personal facts from a conversation exchange.
Return a JSON array of 0-3 short fact strings about the USER only — stable preferences,
biography, projects, people, habits, decisions. Facts must be useful weeks later.
Ignore small talk, one-time requests, and anything already known.
If nothing durable was revealed, return exactly: []
Respond with ONLY the JSON array, nothing else.

Already known:
%s

Exchange:
User: %s
Assistant: %s`

// Learn asks the brain whether the latest exchange revealed durable facts, and stores them.
// Returns the facts it learned (empty if none).
func (m *MemoryStore) Learn(brain *Brain, userMsg, reply string) []string {
	known := "(nothing yet)"
	if n := len(m.Facts); n > 0 {
		var last []string
		for _, f := range m.Facts[max(0, n-20):] {
			last = append(last, f.Text)
		}
		known = strings.Join(last, "; ")
	}
	prompt := fmt.Sprintf(extractPrompt, known, userMsg, reply)
	out, err := brain.Chat([]Message{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return nil
	}
	facts := parseFactArray(out)
	var learned []string
	for _, f := range facts {
		if len(f) < 6 || m.isDuplicate(f) {
			continue
		}
		if m.Remember(f, "auto") == nil {
			learned = append(learned, f)
		}
	}
	return learned
}

// parseFactArray tolerantly finds a JSON string array in model output
// (small models decorate their JSON; we dig it out).
func parseFactArray(out string) []string {
	start := strings.Index(out, "[")
	end := strings.LastIndex(out, "]")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	var arr []string
	if json.Unmarshal([]byte(out[start:end+1]), &arr) != nil {
		return nil
	}
	return arr
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
