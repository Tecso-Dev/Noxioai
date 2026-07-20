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
	"time"
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

	if len(os.Args) > 1 && os.Args[1] == "social" {
		if !socialBrainConfigured() {
			log.Print(socialBrainGuardMessage)
			return
		}
		db := mustDB()
		defer db.Close()
		if err := RunSocial(context.Background(), db); err != nil {
			fmt.Fprintln(os.Stderr, "✗ social:", err)
			os.Exit(1)
		}
		fmt.Printf("✓ %d social drafts stored and delivered for approval\n", socialDraftCount)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "social-approve" {
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: jarvis social-approve <id>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil || id <= 0 {
			fmt.Fprintln(os.Stderr, "usage: jarvis social-approve <id>")
			os.Exit(1)
		}
		db := mustDB()
		defer db.Close()
		result, err := ApproveSocialPost(context.Background(), db, id)
		if err != nil {
			fmt.Fprintln(os.Stderr, "✗ social-approve:", err)
			os.Exit(1)
		}
		switch {
		case result.AlreadyPosted:
			fmt.Printf("✓ social post #%d was already posted\n", id)
		case result.Published:
			fmt.Printf("✓ social post #%d APPROVED and published to Telegram\n", id)
		case result.Platform == "instagram":
			fmt.Printf("✓ social post #%d APPROVED — Instagram-ready; post it manually using official Instagram tools\n", id)
		default:
			fmt.Printf("✓ social post #%d APPROVED — JARVIS_SOCIAL_CHANNEL is not configured; post it manually\n", id)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "social-reject" {
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: jarvis social-reject <id>")
			os.Exit(1)
		}
		id, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil || id <= 0 {
			fmt.Fprintln(os.Stderr, "usage: jarvis social-reject <id>")
			os.Exit(1)
		}
		db := mustDB()
		defer db.Close()
		if err := RejectSocialPost(context.Background(), db, id); err != nil {
			fmt.Fprintln(os.Stderr, "✗ social-reject:", err)
			os.Exit(1)
		}
		fmt.Printf("✓ social post #%d REJECTED\n", id)
		return
	}

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

	if len(os.Args) > 1 && os.Args[1] == "health" {
		db, _ := OpenDB() // db may be nil (Postgres down) — status reports it offline
		if db != nil {
			defer db.Close()
		}
		fmt.Println(renderHealth(collectSystemStatus(context.Background(), db)))
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		db, _ := OpenDB()
		if db != nil {
			defer db.Close()
		}
		s := collectSystemStatus(context.Background(), db)
		problems := evaluateProblems(s)
		changed, newHash := hasStateChanged(readLastHealthHash(), problems)
		if !changed {
			return // silent, exit 0
		}
		writeLastHealthHash(newHash)
		msg := "✅ all clear"
		if len(problems) > 0 {
			msg = "⚠️ JARVIS health problems:\n" + strings.Join(problems, "\n")
		}
		if err := SendTelegram(msg); err != nil {
			fmt.Fprintln(os.Stderr, "✗ healthcheck telegram:", err)
			os.Exit(1)
		}
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

	if len(os.Args) > 1 && os.Args[1] == "madusa" {
		usage := "usage: jarvis madusa <cycle|map|approve <id>|reject <id>|render|status|creators list|creators add <handle>|creators rm <handle>|seed>"
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(1)
		}
		db := mustDB()
		defer db.Close()
		ctx := context.Background()
		switch os.Args[2] {
		case "cycle":
			if err := RunMadusaCycle(ctx, db, NewBrainFromEnv(), mustOwnerID(db)); err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa cycle:", err)
				os.Exit(1)
			}
			fmt.Println("✓ MADUSA cycle complete")
		case "map":
			if err := MadusaMap(ctx, db); err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa map:", err)
				os.Exit(1)
			}
			fmt.Println("✓ MADUSA map re-sent")
		case "approve":
			if len(os.Args) != 4 {
				fmt.Fprintln(os.Stderr, "usage: jarvis madusa approve <id>")
				os.Exit(1)
			}
			id, err := strconv.ParseInt(os.Args[3], 10, 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "usage: jarvis madusa approve <id>")
				os.Exit(1)
			}
			if err := MadusaApprove(ctx, db, id); err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa approve:", err)
				os.Exit(1)
			}
			fmt.Printf("✓ post #%d approved — render timer will pick it up within 15 min; run 'jarvis madusa render' to start now\n", id)
		case "reject":
			if len(os.Args) != 4 {
				fmt.Fprintln(os.Stderr, "usage: jarvis madusa reject <id>")
				os.Exit(1)
			}
			id, err := strconv.ParseInt(os.Args[3], 10, 64)
			if err != nil {
				fmt.Fprintln(os.Stderr, "usage: jarvis madusa reject <id>")
				os.Exit(1)
			}
			if err := MadusaReject(ctx, db, id); err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa reject:", err)
				os.Exit(1)
			}
			fmt.Printf("✓ post #%d rejected\n", id)
		case "render":
			if err := MadusaRender(ctx, db, NewBrainFromEnv()); err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa render:", err)
				os.Exit(1)
			}
			fmt.Println("✓ MADUSA render complete")
		case "status":
			s, err := MadusaStatus(ctx, db)
			if err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa status:", err)
				os.Exit(1)
			}
			fmt.Println(s)
		case "creators":
			if len(os.Args) < 4 {
				fmt.Fprintln(os.Stderr, usage)
				os.Exit(1)
			}
			switch os.Args[3] {
			case "list":
				s, err := MadusaCreatorList(ctx, db)
				if err != nil {
					fmt.Fprintln(os.Stderr, "✗ madusa creators list:", err)
					os.Exit(1)
				}
				fmt.Println(s)
			case "add":
				if len(os.Args) != 5 {
					fmt.Fprintln(os.Stderr, "usage: jarvis madusa creators add <handle>")
					os.Exit(1)
				}
				if err := MadusaCreatorAdd(ctx, db, os.Args[4]); err != nil {
					fmt.Fprintln(os.Stderr, "✗ madusa creators add:", err)
					os.Exit(1)
				}
				fmt.Printf("✓ creator %s added\n", os.Args[4])
			case "rm":
				if len(os.Args) != 5 {
					fmt.Fprintln(os.Stderr, "usage: jarvis madusa creators rm <handle>")
					os.Exit(1)
				}
				if err := MadusaCreatorRemove(ctx, db, os.Args[4]); err != nil {
					fmt.Fprintln(os.Stderr, "✗ madusa creators rm:", err)
					os.Exit(1)
				}
				fmt.Printf("✓ creator %s removed\n", os.Args[4])
			default:
				fmt.Fprintln(os.Stderr, usage)
				os.Exit(1)
			}
		case "seed":
			if err := MadusaSeedCreators(ctx, db); err != nil {
				fmt.Fprintln(os.Stderr, "✗ madusa seed:", err)
				os.Exit(1)
			}
			fmt.Println("✓ MADUSA creators seeded")
		default:
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(1)
		}
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
	server := &http.Server{
		Addr:              addr,
		Handler:           authSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		// Chat responses stream over SSE, so keep the write timeout long enough
		// for model output while still bounding abandoned connections.
		WriteTimeout:   2 * time.Minute,
		IdleTimeout:    90 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// chatRow is one persisted admin-console turn, as read from chat_messages.
type chatRow struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

// ChatTurn is the JSON shape GET /api/chat/history returns.
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Ts      string `json:"ts"`
}

// shapeChatHistory turns newest-first DB rows into oldest-first JSON turns.
func shapeChatHistory(rows []chatRow) []ChatTurn {
	turns := make([]ChatTurn, len(rows))
	for i, r := range rows {
		turns[len(rows)-1-i] = ChatTurn{Role: r.Role, Content: r.Content, Ts: r.CreatedAt.Format(time.RFC3339)}
	}
	return turns
}

func registerChat(mux *http.ServeMux, brain *Brain, db *sql.DB) {
	mux.HandleFunc("POST /chat", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		var req struct {
			Messages []Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Owner decision 2026-07-20: the admin console gets JARVIS's personal
		// memory (profile + learned facts), same as the CLI REPL. Tenant bot
		// and support paths never call LoadMemory/SystemContext — this stays
		// admin-only.
		memory := LoadMemory()
		system := systemPrompt + memory.SystemContext() +
			"\n\n## Live business data (real, current — answer from THIS, never invent)\n" + crmSnapshot(r.Context(), db, ownerID) +
			ceoSystemPromptSection
		history := append([]Message{{Role: "system", Content: system}}, req.Messages...)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		filter := newSSELineFilter(w, flusher)
		reply, err := brain.Chat(history, filter.Token)
		filter.Flush()
		if err != nil {
			fmt.Fprintf(w, "data: %s\n\n", `{"error":"brain error"}`)
		} else {
			var userMsg string
			if n := len(req.Messages); n > 0 {
				userMsg = req.Messages[n-1].Content
			}
			strippedReply := stripCEOCommandLines(reply)
			if err := saveChatMessage(r.Context(), db, ownerID, "user", userMsg); err != nil {
				log.Printf("chat history save (user) failed: %v", err)
			}
			if err := saveChatMessage(r.Context(), db, ownerID, "assistant", strippedReply); err != nil {
				log.Printf("chat history save (assistant) failed: %v", err)
			}
			memory.Learn(brain, userMsg, strippedReply)

			if cmds := parseCEOCommands(reply); len(cmds) > 0 {
				runCEOCommands(db, brain, ownerID, cmds)
				writeSSEToken(w, flusher, ceoDispatchSummary(cmds))
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	})
}

// saveChatMessage persists one turn of the admin console conversation.
// Best-effort only — a storage hiccup must never break the live SSE stream.
func saveChatMessage(ctx context.Context, db *sql.DB, ownerID int64, role, content string) error {
	if db == nil || content == "" {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO chat_messages (owner_id, role, content) VALUES ($1, $2, $3)`,
		ownerID, role, content)
	return err
}
