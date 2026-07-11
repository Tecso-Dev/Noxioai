package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const systemPrompt = `You are JARVIS, Sobhan's personal AI assistant — calm, precise,
lightly witty, in the spirit of a perfect British butler. Address him as "Sir"
occasionally, never excessively. Answer in the language he speaks to you:
English or Persian (فارسی). Keep spoken-style answers concise — you will
eventually be read aloud. If asked to do something you cannot do yet, say so
plainly and suggest what you could do instead.`

func main() {
	if len(os.Args) > 2 && os.Args[1] == "db" && os.Args[2] == "init" {
		db, err := OpenDB()
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ cannot reach Postgres:", err)
			os.Exit(1)
		}
		defer db.Close()
		if err := InitSchema(context.Background(), db); err != nil {
			fmt.Fprintln(os.Stderr, "✗ schema failed:", err)
			os.Exit(1)
		}
		fmt.Println("✓ schema applied: companies, contacts, leads, outreach, experiences")
		return
	}

	brain := NewBrainFromEnv()
	memory := LoadMemory()
	fmt.Printf("⚡ JARVIS v0.2 — brain: %s (%s) — memory: %d facts\n", brain.Model, brain.BaseURL, len(memory.Facts))

	if len(os.Args) > 1 && os.Args[1] == "serve" {
		serveHTTP(brain, memory)
		return
	}
	repl(brain, memory)
}

// repl is the terminal conversation loop — now with a memory that learns Sobhan.
func repl(brain *Brain, memory *MemoryStore) {
	history := []Message{{Role: "system", Content: systemPrompt + memory.SystemContext()}}
	fmt.Println("Type your message ('exit', '/memory', '/remember <fact>', '/forget <text>'):")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch {
		case input == "exit" || input == "quit":
			fmt.Println("JARVIS: Goodbye, Sir.")
			return
		case input == "/memory":
			fmt.Printf("JARVIS knows %d facts:\n", len(memory.Facts))
			for _, f := range memory.Facts {
				fmt.Printf("  • %s  (%s, %s)\n", f.Text, f.Source, f.Learned.Format("Jan 2"))
			}
			continue
		case strings.HasPrefix(input, "/remember "):
			memory.Remember(strings.TrimPrefix(input, "/remember "), "explicit")
			fmt.Println("JARVIS: Noted, Sir. I will remember that.")
			continue
		case strings.HasPrefix(input, "/forget "):
			n := memory.Forget(strings.TrimPrefix(input, "/forget "))
			fmt.Printf("JARVIS: Removed %d memories, Sir.\n", n)
			continue
		}

		history = append(history, Message{Role: "user", Content: input})
		fmt.Print("JARVIS: ")
		reply, err := brain.Chat(history, func(tok string) { fmt.Print(tok) })
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [error] %v\n", err)
			history = history[:len(history)-1]
			continue
		}
		history = append(history, Message{Role: "assistant", Content: reply})

		// the learning loop: quietly extract durable facts from this exchange
		if learned := memory.Learn(brain, input, reply); len(learned) > 0 {
			for _, f := range learned {
				fmt.Printf("  💡 [learned] %s\n", f)
			}
		}
	}
}

// serveHTTP exposes the SSE endpoint the HUD (Step 6) will consume.
func serveHTTP(brain *Brain, memory *MemoryStore) {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		history := append([]Message{{Role: "system", Content: systemPrompt + memory.SystemContext()}}, req.Messages...)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		_, err := brain.Chat(history, func(tok string) {
			data, _ := json.Marshal(map[string]string{"token": tok})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		})
		if err != nil {
			fmt.Fprintf(w, "data: %s\n\n", `{"error":"brain error"}`)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "online", "model": brain.Model})
	})

	addr := envOr("JARVIS_ADDR", "127.0.0.1:7700")
	fmt.Printf("HTTP up on http://%s  (POST /chat, GET /health)\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
