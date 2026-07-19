package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const systemPrompt = `You are JARVIS, Sobhan's personal AI assistant — calm, precise,
lightly witty, in the spirit of a perfect British butler. Address him as "Sir"
occasionally, never excessively. Answer in the language he speaks to you:
English or Persian (فارسی). Keep spoken-style answers concise — you will
eventually be read aloud. If asked to do something you cannot do yet, say so
plainly and suggest what you could do instead. For brief acknowledgements and
status updates, vary with original, composed system language such as "Systems
are standing by, Sir" or "Diagnostics are green"; never quote film dialogue
or imitate a real actor.`

// mustOwnerID resolves the CLI owner (JARVIS_OWNER_EMAIL, default Sobhan) or
// exits — every CRM CLI command needs one (PRODUCT-BUILD.md Phase P1).
func mustOwnerID(db *sql.DB) int64 {
	id, err := defaultOwnerID(context.Background(), db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗ cannot resolve owner (run `jarvis db init` first):", err)
		os.Exit(1)
	}
	return id
}

func main() {
	loadDotEnv()

	if len(os.Args) > 1 && os.Args[1] == "seo" {
		_, enabled, err := seoServiceAccountPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ seo:", err)
			os.Exit(1)
		}
		if !enabled {
			log.Print(seoGuardMessage)
			return
		}
		db := mustDB()
		defer db.Close()
		if err := RunSEO(context.Background(), db); err != nil {
			fmt.Fprintln(os.Stderr, "✗ seo:", err)
			os.Exit(1)
		}
		fmt.Println("✓ SEO report stored and delivered to Telegram")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "support" {
		if strings.TrimSpace(os.Getenv("JARVIS_SUPPORT_BOT_TOKEN")) == "" {
			fmt.Println("support bot token not configured")
			return
		}
		db := mustDB()
		defer db.Close()
		RunSupportBot(context.Background(), db)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "brief" {
		db := mustDB()
		defer db.Close()
		ownerID := mustOwnerID(db)
		if err := RunBrief(context.Background(), db, ownerID, NewBrainFromEnv()); err != nil {
			fmt.Fprintln(os.Stderr, "✗ brief:", err)
			os.Exit(1)
		}
		fmt.Println("✓ briefing delivered to Telegram")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "inbox" {
		db := mustDB()
		defer db.Close()
		ownerID := mustOwnerID(db)
		replies, err := CheckInbox(context.Background(), db, ownerID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ inbox:", err)
			os.Exit(1)
		}
		fmt.Printf("📬 %d replies processed\n", replies)
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "db" && os.Args[2] == "init" {
		db, err := OpenDB()
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ cannot reach Postgres:", err)
			os.Exit(1)
		}
		defer db.Close()
		ownerID, err := InitSchema(context.Background(), db)
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ schema failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✓ schema applied: companies, contacts, leads, outreach, experiences\n✓ owner ready: user #%d (existing rows backfilled)\n", ownerID)
		return
	}

	if len(os.Args) > 2 && (os.Args[1] == "oracle" || os.Args[1] == "atlas") {
		db := mustDB()
		defer db.Close()
		ownerID := mustOwnerID(db)
		var agent Agent
		if os.Args[1] == "oracle" {
			agent = &Oracle{Brain: NewBrainFromEnv(), DB: db, OwnerID: ownerID}
		} else {
			agent = &Atlas{Brain: NewBrainFromEnv(), DB: db, OwnerID: ownerID}
		}
		res, err := agent.Run(context.Background(), Task{Agent: os.Args[1], Input: strings.Join(os.Args[2:], " ")})
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", os.Args[1], err)
			os.Exit(1)
		}
		fmt.Println("⚡", res.Output)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "leads" {
		db := mustDB()
		defer db.Close()
		if err := PrintLeads(context.Background(), db, mustOwnerID(db)); err != nil {
			fmt.Fprintln(os.Stderr, "✗ leads:", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "approve" {
		db := mustDB()
		defer db.Close()
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: jarvis approve <outreach-id>")
			os.Exit(1)
		}
		draft, err := ApproveOutreach(context.Background(), db, mustOwnerID(db), id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ approve:", err)
			os.Exit(1)
		}
		fmt.Printf("✓ outreach #%d APPROVED — copy & send, then record with `jarvis outcome %d <sent|replied|meeting|won|lost>`\n\n%s\n", id, id, draft)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "caleb" {
		db := mustDB()
		defer db.Close()
		memo, err := RunCaleb(context.Background(), db, mustOwnerID(db), NewBrainFromEnv())
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ caleb:", err)
			os.Exit(1)
		}
		fmt.Println("📈 CALEB — marketing memo:\n\n" + memo)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "followup" {
		db := mustDB()
		defer db.Close()
		drafted, err := RunFollowup(context.Background(), db, mustOwnerID(db), NewBrainFromEnv())
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ followup:", err)
			os.Exit(1)
		}
		fmt.Printf("✍️ %d follow-ups drafted (unapproved)\n", drafted)
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "pixel" {
		db := mustDB()
		defer db.Close()
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: jarvis pixel <lead-id>")
			os.Exit(1)
		}
		critique, err := RunPixel(context.Background(), db, mustOwnerID(db), NewBrainFromEnv(), id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ pixel:", err)
			os.Exit(1)
		}
		fmt.Println("🎨 PIXEL — design critique:\n\n" + critique)
		return
	}

	if len(os.Args) > 2 && os.Args[1] == "send" {
		db := mustDB()
		defer db.Close()
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: jarvis send <approved-email-outreach-id>")
			os.Exit(1)
		}
		to, err := HeraldSend(context.Background(), db, mustOwnerID(db), id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ HERALD:", err)
			os.Exit(1)
		}
		fmt.Printf("✉️ HERALD dispatched outreach #%d to %s\n", id, to)
		return
	}

	if len(os.Args) > 3 && os.Args[1] == "outcome" {
		db := mustDB()
		defer db.Close()
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: jarvis outcome <outreach-id> <sent|no_reply|replied|meeting|won|lost>")
			os.Exit(1)
		}
		if err := SetOutcome(context.Background(), db, mustOwnerID(db), id, os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "✗ outcome:", err)
			os.Exit(1)
		}
		fmt.Printf("✓ outcome %q recorded for outreach #%d\n", os.Args[3], id)
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

// serveHTTP serves the Iron-Man HUD plus the SSE chat endpoint it consumes.
func serveHTTP(brain *Brain, memory *MemoryStore) {
	mux := http.NewServeMux()

	db, err := OpenDB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "⚠ CRM offline (HUD panels degraded):", err)
		db = nil
	}
	registerHUD(mux, brain, memory, db)
	registerAuth(mux, db)
	registerProfile(mux, db)
	registerTenantBot(mux, db, brain)
	registerConcierge(mux, db, brain)
	registerWaitlist(mux, db)
	registerBilling(mux, db)
	registerChat(mux, brain, db)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "online", "model": brain.Model})
	})

	addr := envOr("JARVIS_ADDR", "127.0.0.1:7700")
	fmt.Printf("⚡ JARVIS HUD on http://%s  (dashboard, POST /chat, GET /health)\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func registerChat(mux *http.ServeMux, brain *Brain, db *sql.DB) {
	mux.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "CRM offline", http.StatusServiceUnavailable)
			return
		}
		ownerID, ok := sessionOwner(db, w, r)
		if !ok {
			return
		}
		var req struct {
			Messages []Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Personal JARVIS memory remains CLI-only until P2 introduces tenant
		// business profiles. Web chat receives only this tenant's CRM snapshot.
		system := systemPrompt + "\n\n## Live business data (real, current — answer from THIS, never invent)\n" + crmSnapshot(r.Context(), db, ownerID)
		history := append([]Message{{Role: "system", Content: system}}, req.Messages...)

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
}
