package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// busySet tracks which agents are mid-task so the workspace can show it.
type busySet struct {
	mu sync.Mutex
	m  map[string]bool
}

func (b *busySet) set(name string, v bool) { b.mu.Lock(); b.m[name] = v; b.mu.Unlock() }
func (b *busySet) get(name string) bool    { b.mu.Lock(); defer b.mu.Unlock(); return b.m[name] }

//go:embed web/hud.html
var hudHTML []byte

//go:embed web/three.min.js
var threeJS []byte

//go:embed web/jarvis-startup.mp3
var startupSFX []byte

// activity is the HUD's in-memory event feed (last 50 lines, lost on restart —
// the durable record lives in the experiences table).
type activity struct {
	mu    sync.Mutex
	lines []string
	total int // monotonic counter so the HUD can toast only NEW events
}

func (a *activity) add(format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.total++
	a.lines = append(a.lines, time.Now().Format("15:04:05")+"  "+fmt.Sprintf(format, args...))
	if len(a.lines) > 50 {
		a.lines = a.lines[len(a.lines)-50:]
	}
}

func (a *activity) seq() int { a.mu.Lock(); defer a.mu.Unlock(); return a.total }

func (a *activity) snapshot() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.lines...)
}

// heraldStatus: "ready" once the Gmail app password is in .env, else standby.
func heraldStatus() string {
	if os.Getenv("JARVIS_SMTP_PASS") != "" {
		return "ready"
	}
	return "v2"
}

// registerHUD wires the Iron-Man dashboard onto the serve mux. db may be nil
// (Postgres down) — the HUD still chats, panels show CRM OFFLINE.
func registerHUD(mux *http.ServeMux, brain *Brain, memory *MemoryStore, db *sql.DB) {
	act := &activity{}
	act.add("JARVIS HUD online — brain %s", brain.Model)
	started := time.Now()
	busy := &busySet{m: map[string]bool{}}

	mux.HandleFunc("GET /three.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, err := w.Write(threeJS); err != nil {
			return
		}
	})

	// The startup mix is embedded so the local HUD remains fully self-contained.
	mux.HandleFunc("GET /jarvis-startup.mp3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		if _, err := w.Write(startupSFX); err != nil {
			return
		}
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(hudHTML); err != nil {
			return
		}
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		lastOps := map[string]string{}
		if db != nil {
			if rows, err := db.QueryContext(ctx, `
				SELECT DISTINCT ON (agent) agent, decision || ' · ' || to_char(created_at,'HH24:MI')
				FROM experiences ORDER BY agent, created_at DESC`); err == nil {
				for rows.Next() {
					var a, d string
					if rows.Scan(&a, &d) == nil {
						lastOps[a] = d
					}
				}
				rows.Close()
			}
		}
		mkAgent := func(name, role, st string) map[string]string {
			if busy.get(name) {
				st = "busy"
			}
			return map[string]string{"name": name, "role": role, "status": st,
				"last": lastOps[strings.ToLower(name)]}
		}
		resp := map[string]any{
			"model":   brain.Model,
			"facts":   len(memory.Facts),
			"balance": deepseekBalance(),
			"uptime":  time.Since(started).Round(time.Minute).String(),
			"db":      "offline",
			"agents": []map[string]string{
				mkAgent("ORACLE", "Market intelligence", "ready"),
				mkAgent("ATLAS", "Outreach drafting", "ready"),
				mkAgent("FRIDAY", "Daily briefing 08:00", "scheduled"),
				mkAgent("CALEB", "Marketing strategist", "ready"),
				mkAgent("PIXEL", "Design & motion critic", "v2"),
				mkAgent("HERALD", "Email dispatch", heraldStatus()),
			},
			"activity": act.snapshot(),
			"act_seq":  act.seq(),
		}
		if db != nil && db.PingContext(ctx) == nil {
			resp["db"] = "online"
			var leadCount, pendingCount, approvedCount, expCount int
			db.QueryRowContext(ctx, `SELECT count(*) FROM leads`).Scan(&leadCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM outreach WHERE NOT approved AND outcome IS NULL`).Scan(&pendingCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM outreach WHERE approved`).Scan(&approvedCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM experiences`).Scan(&expCount)
			resp["lead_count"], resp["pending_count"] = leadCount, pendingCount
			resp["approved_count"], resp["exp_count"] = approvedCount, expCount
			var contactCount int
			db.QueryRowContext(ctx, `SELECT count(*) FROM contacts`).Scan(&contactCount)
			resp["contact_count"] = contactCount
			groupCount := func(query string) map[string]int {
				out := map[string]int{}
				if rows, err := db.QueryContext(ctx, query); err == nil {
					for rows.Next() {
						var k string
						var n int
						if rows.Scan(&k, &n) == nil {
							out[k] = n
						}
					}
					rows.Close()
				}
				return out
			}
			resp["funnel"] = groupCount(`SELECT status, count(*) FROM leads GROUP BY status`)
			resp["tiers"] = groupCount(`SELECT COALESCE(tier,'?'), count(*) FROM leads GROUP BY 1`)

			leads := []map[string]any{}
			if rows, err := db.QueryContext(ctx, `
				SELECT l.id, COALESCE(l.score,0), COALESCE(l.tier,''), l.status, c.name,
				       EXISTS(SELECT 1 FROM contacts ct WHERE ct.company_id=c.id AND COALESCE(ct.email,'')<>'') AS has_email
				FROM leads l JOIN companies c ON c.id=l.company_id ORDER BY l.score DESC LIMIT 20`); err == nil {
				for rows.Next() {
					var id int64
					var score int
					var tier, status, name string
					var hasEmail bool
					if rows.Scan(&id, &score, &tier, &status, &name, &hasEmail) == nil {
						leads = append(leads, map[string]any{"id": id, "score": score, "tier": tier, "status": status, "name": name, "has_email": hasEmail})
					}
				}
				rows.Close()
			}
			resp["leads"] = leads

			drafts := []map[string]any{}
			if rows, err := db.QueryContext(ctx, `
				SELECT o.id, o.channel, c.name
				FROM outreach o JOIN leads l ON l.id=o.lead_id JOIN companies c ON c.id=l.company_id
				WHERE NOT o.approved AND o.outcome IS NULL ORDER BY o.id`); err == nil {
				for rows.Next() {
					var id int64
					var channel, name string
					if rows.Scan(&id, &channel, &name) == nil {
						drafts = append(drafts, map[string]any{"id": id, "channel": channel, "company": name})
					}
				}
				rows.Close()
			}
			resp["drafts"] = drafts
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Agent dossier for the workspace popups: mission history from experiences.
	mux.HandleFunc("GET /api/agent", func(w http.ResponseWriter, r *http.Request) {
		name := strings.ToLower(r.URL.Query().Get("name"))
		if db == nil || name == "" {
			http.Error(w, "need ?name and a live CRM", http.StatusBadRequest)
			return
		}
		missions := []map[string]string{}
		if rows, err := db.QueryContext(r.Context(), `
			SELECT to_char(created_at,'DD Mon HH24:MI'), COALESCE(input,''), COALESCE(decision,''),
			       COALESCE(result,''), COALESCE(lesson,'')
			FROM experiences WHERE agent=$1 ORDER BY created_at DESC LIMIT 10`, name); err == nil {
			for rows.Next() {
				var when, in, dec, res, les string
				if rows.Scan(&when, &in, &dec, &res, &les) == nil {
					missions = append(missions, map[string]string{
						"when": when, "input": in, "decision": dec, "result": res, "lesson": les})
				}
			}
			rows.Close()
		}
		var count int
		db.QueryRowContext(r.Context(), `SELECT count(*) FROM experiences WHERE agent=$1`, name).Scan(&count)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"name": name, "count": count, "missions": missions})
	})

	runAgent := func(a Agent, input, started string) {
		name := strings.ToUpper(a.Name())
		act.add("%s", started)
		busy.set(name, true)
		go func() {
			defer busy.set(name, false)
			res, err := a.Run(context.Background(), Task{Agent: a.Name(), Input: input})
			if err != nil {
				act.add("✗ %s: %v", a.Name(), err)
				return
			}
			act.add("✓ %s: %s", a.Name(), res.Output)
		}()
	}

	mux.HandleFunc("POST /api/oracle", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Niche string `json:"niche"` }
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Niche == "" || db == nil {
			http.Error(w, "need {niche} and a live CRM", http.StatusBadRequest)
			return
		}
		runAgent(&Oracle{Brain: brain, DB: db}, req.Niche, "🔎 ORACLE hunting: "+req.Niche)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /api/atlas", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Lead int64 `json:"lead"` }
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Lead == 0 || db == nil {
			http.Error(w, "need {lead} and a live CRM", http.StatusBadRequest)
			return
		}
		runAgent(&Atlas{Brain: brain, DB: db}, fmt.Sprint(req.Lead), fmt.Sprintf("✍️ ATLAS drafting for lead %d", req.Lead))
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /api/inbox", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "CRM offline", http.StatusServiceUnavailable)
			return
		}
		act.add("📬 HERALD checking inbox for replies")
		busy.set("HERALD", true)
		go func() {
			defer busy.set("HERALD", false)
			n, err := CheckInbox(context.Background(), db)
			if err != nil {
				act.add("✗ inbox: %v", err)
				return
			}
			act.add("✓ inbox: %d replies processed", n)
		}()
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /api/brief", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "CRM offline", http.StatusServiceUnavailable)
			return
		}
		act.add("📨 briefing requested")
		busy.set("FRIDAY", true)
		busy.set("CALEB", true)
		go func() {
			defer busy.set("FRIDAY", false)
			defer busy.set("CALEB", false)
			if err := RunBrief(context.Background(), db, brain); err != nil {
				act.add("✗ brief: %v", err)
			} else {
				act.add("✓ brief delivered to Telegram")
			}
		}()
		w.WriteHeader(http.StatusAccepted)
	})

	// HERALD dispatch — only works on drafts Sobhan already approved.
	mux.HandleFunc("POST /api/send", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ ID int64 `json:"id"` }
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == 0 || db == nil {
			http.Error(w, "need {id} and a live CRM", http.StatusBadRequest)
			return
		}
		to, err := HeraldSend(r.Context(), db, req.ID)
		if err != nil {
			act.add("✗ HERALD: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		act.add("✉️ HERALD dispatched #%d to %s", req.ID, to)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"to": to})
	})

	// Approve stays synchronous — it IS the human gate (Principle 1).
	mux.HandleFunc("POST /api/approve", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ ID int64 `json:"id"` }
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == 0 || db == nil {
			http.Error(w, "need {id} and a live CRM", http.StatusBadRequest)
			return
		}
		draft, err := ApproveOutreach(r.Context(), db, req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		act.add("✅ outreach #%d approved by Sobhan", req.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"draft": draft})
	})
}
