package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Brain speaks the OpenAI-compatible chat API — the shared wire format of
// Ollama (local), DeepSeek, Qwen/DashScope and OpenRouter. Switching brains
// is configuration, not code.
type Brain struct {
	BaseURL string
	APIKey  string
	Model   string
	client  *http.Client
}

func NewBrainFromEnv() *Brain {
	b := &Brain{
		BaseURL: envOr("JARVIS_BASE_URL", "http://localhost:11434/v1"),
		APIKey:  os.Getenv("JARVIS_API_KEY"), // empty is fine for local Ollama
		Model:   envOr("JARVIS_MODEL", "qwen2.5:3b"),
		client:  &http.Client{Timeout: 5 * time.Minute},
	}
	return b
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// streamed response chunk (OpenAI SSE format)
type chunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// Chat streams the assistant reply, calling onToken for each token.
// Returns the full reply text.
func (b *Brain) Chat(history []Message, onToken func(string)) (string, error) {
	payload, err := json.Marshal(chatRequest{Model: b.Model, Messages: history, Stream: true})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", strings.TrimRight(b.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("brain unreachable at %s: %w", b.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body bytes.Buffer
		body.ReadFrom(resp.Body)
		return "", fmt.Errorf("brain returned %s: %s", resp.Status, strings.TrimSpace(body.String()))
	}

	var full strings.Builder
	filter := &thoughtFilter{emit: onToken}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var c chunk
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue
		}
		if len(c.Choices) > 0 && c.Choices[0].Delta.Content != "" {
			token := c.Choices[0].Delta.Content
			full.WriteString(token)
			if onToken != nil {
				filter.feed(token)
			}
		}
	}
	return stripThought(full.String()), scanner.Err()
}

// Gemma 4 (via the Gemini API) prefixes replies with a <thought>…</thought>
// reasoning block. Strip it from both the token stream and the returned text
// so it never leaks into drafts or the HUD.
const thoughtOpen, thoughtClose = "<thought>", "</thought>"

func stripThought(s string) string {
	t := strings.TrimLeft(s, " \t\n")
	if !strings.HasPrefix(t, thoughtOpen) {
		return s
	}
	if i := strings.Index(t, thoughtClose); i >= 0 {
		return strings.TrimLeft(t[i+len(thoughtClose):], " \n")
	}
	return s
}

// thoughtFilter applies stripThought logic to a token stream, where the tags
// may arrive split across tokens.
type thoughtFilter struct {
	emit  func(string)
	buf   strings.Builder
	state int // 0 = deciding, 1 = inside thought block, 2 = passing through
}

func (f *thoughtFilter) feed(tok string) {
	switch f.state {
	case 2:
		f.emit(tok)
	case 0:
		f.buf.WriteString(tok)
		s := strings.TrimLeft(f.buf.String(), " \t\n")
		if strings.HasPrefix(s, thoughtOpen) {
			f.state = 1
			f.scanClose()
		} else if !strings.HasPrefix(thoughtOpen, s) { // no longer a possible prefix
			f.state = 2
			f.emit(f.buf.String())
		}
	case 1:
		f.buf.WriteString(tok)
		f.scanClose()
	}
}

func (f *thoughtFilter) scanClose() {
	if i := strings.Index(f.buf.String(), thoughtClose); i >= 0 {
		rest := strings.TrimLeft(f.buf.String()[i+len(thoughtClose):], " \n")
		f.state = 2
		if rest != "" {
			f.emit(rest)
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
