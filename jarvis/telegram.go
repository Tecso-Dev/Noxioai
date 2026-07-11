package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// loadDotEnv reads jarvis/.env (gitignored — tokens live there, never in git).
// Real environment variables win over file values.
func loadDotEnv() {
	for _, p := range []string{
		filepath.Join(os.Getenv("HOME"), "Documents", "noxioai", "jarvis", ".env"),
		".env",
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if k, v, ok := strings.Cut(line, "="); ok && os.Getenv(k) == "" {
				os.Setenv(k, strings.TrimSpace(v))
			}
		}
	}
}

func SendTelegram(text string) error {
	token := os.Getenv("JARVIS_TELEGRAM_TOKEN")
	chat := os.Getenv("JARVIS_TELEGRAM_CHAT")
	if token == "" || chat == "" {
		return fmt.Errorf("JARVIS_TELEGRAM_TOKEN / JARVIS_TELEGRAM_CHAT not set (jarvis/.env)")
	}
	resp, err := http.PostForm("https://api.telegram.org/bot"+token+"/sendMessage",
		url.Values{"chat_id": {chat}, "text": {text}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("telegram %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return nil
}
