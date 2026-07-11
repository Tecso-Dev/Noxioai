package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

//go:embed web/hud.html
var hudHTML []byte

// activity is the HUD's in-memory event feed (last 50 lines, lost on restart —
// the durable record lives in the experiences table).
type activity struct {
	mu    sync.Mutex
	lines []string
}

func (a *activity) add(format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lines = append(a.lines, time.Now().Format("15:04:05")+"  "+fmt.Sprintf(format, args...))
	if len(a.lines) > 50 {
		a.lines = a.lines[len(a.lines)-50:]
	}
}

func (a *activity) snapshot() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.lines...)
}

// registerHUD wires the Iron-Man dashboard onto the serve mux. db may be nil
// (Postgres down) — the HUD still chats, panels show CRM OFFLINE.
func registerHUD(mux *http.ServeMux, brain *Brain, memory *MemoryStore, db *sql.DB) {
	act := &activity{}
	act.add("JARVIS HUD online — brain %s", brain.Model)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(hudHTML)
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		resp := map[string]any{
			"model":   brain.Model,
			"facts":   len(memory.Facts),
			"balance": deepseekBalance(),
			"db":      "offline",
			"agents": []map[string]string{
				{"name": "ORACLE", "role": "Market intelligence", "status": "ready"},
				{"name": "ATLAS", "role": "Outreach drafting", "status": "ready"},
				{"name": "FRIDAY", "role": "Daily briefing 08:00", "status": "scheduled"},
				{"name": "CALEB", "role": "Marketing strategist", "status": "v2"},
				{"name": "PIXEL", "role": "Design & motion critic", "status": "v2"},
				{"name": "HERALD", "role": "Publisher & inbox", "status": "v2"},
			},
			"activity": act.snapshot(),
		}
		if db != nil && db.PingContext(ctx) == nil {
			resp["db"] = "online"
			var leadCount, pendingCount int
			db.QueryRowContext(ctx, `SELECT count(*) FROM leads`).Scan(&leadCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM outreach WHERE NOT approved AND outcome IS NULL`).Scan(&pendingCount)
			resp["lead_count"], resp["pending_count"] = leadCount, pendingCount

			leads := []map[string]any{}
			if rows, err := db.QueryContext(ctx, `
				SELECT l.id, COALESCE(l.score,0), COALESCE(l.tier,''), l.status, c.name
				FROM leads l JOIN companies c ON c.id=l.company_id ORDER BY l.score DESC LIMIT 20`); err == nil {
				for rows.Next() {
					var id int64
					var score int
					var tier, status, name string
					if rows.Scan(&id, &score, &tier, &status, &name) == nil {
						leads = append(leads, map[string]any{"id": id, "score": score, "tier": tier, "status": status, "name": name})
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

	runAgent := func(a Agent, input, started string) {
		act.add("%s", started)
		go func() {
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

	mux.HandleFunc("POST /api/brief", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "CRM offline", http.StatusServiceUnavailable)
			return
		}
		act.add("📨 briefing requested")
		go func() {
			if err := RunBrief(context.Background(), db, brain); err != nil {
				act.add("✗ brief: %v", err)
			} else {
				act.add("✓ brief delivered to Telegram")
			}
		}()
		w.WriteHeader(http.StatusAccepted)
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
