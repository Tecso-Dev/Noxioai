package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
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

var errAuthenticationRequired = errors.New("authentication required")

// ownerFromSession is the single HTTP authorization boundary for tenant CRM
// data. CLI commands use defaultOwnerID; HTTP requests never inherit it.
func ownerFromSession(ctx context.Context, db *sql.DB, r *http.Request) (int64, error) {
	user, err := currentUser(ctx, db, r)
	if err != nil {
		return 0, err
	}
	if user == nil || !user.Verified {
		return 0, errAuthenticationRequired
	}
	return user.ID, nil
}

// sessionOwner writes an appropriate HTTP error so handlers can return on a
// failed owner lookup without confusing database failures with a bad session.
func sessionOwner(db *sql.DB, w http.ResponseWriter, r *http.Request) (ownerID int64, ok bool) {
	ownerID, err := ownerFromSession(r.Context(), db, r)
	if errors.Is(err, errAuthenticationRequired) {
		http.Error(w, "unauthorized: sign in required", http.StatusUnauthorized)
		return 0, false
	}
	if err != nil {
		http.Error(w, "owner lookup failed", http.StatusInternalServerError)
		return 0, false
	}
	return ownerID, true
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

	// The public HUD entry point (noxioai.com/admin). Gated by is_admin, not
	// a secret URL — a human loading the page without a valid admin session
	// is bounced to /login rather than shown a bare JSON error.
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := resolveAdmin(r, db); !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(hudHTML); err != nil {
			return
		}
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		ctx := r.Context()
		ownerID, err := defaultOwnerID(ctx, db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		lastOps := map[string]string{}
		if db != nil {
			if rows, err := db.QueryContext(ctx, `
				SELECT DISTINCT ON (agent) agent, decision || ' · ' || to_char(created_at,'HH24:MI')
				FROM experiences WHERE owner_id=$1 ORDER BY agent, created_at DESC`, ownerID); err == nil {
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
			db.QueryRowContext(ctx, `SELECT count(*) FROM leads WHERE owner_id=$1`, ownerID).Scan(&leadCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM outreach WHERE owner_id=$1 AND NOT approved AND outcome IS NULL`, ownerID).Scan(&pendingCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM outreach WHERE owner_id=$1 AND approved`, ownerID).Scan(&approvedCount)
			db.QueryRowContext(ctx, `SELECT count(*) FROM experiences WHERE owner_id=$1`, ownerID).Scan(&expCount)
			resp["lead_count"], resp["pending_count"] = leadCount, pendingCount
			resp["approved_count"], resp["exp_count"] = approvedCount, expCount
			var contactCount int
			db.QueryRowContext(ctx, `SELECT count(*) FROM contacts WHERE owner_id=$1`, ownerID).Scan(&contactCount)
			resp["contact_count"] = contactCount
			groupCount := func(query string) map[string]int {
				out := map[string]int{}
				if rows, err := db.QueryContext(ctx, query, ownerID); err == nil {
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
			resp["funnel"] = groupCount(`SELECT status, count(*) FROM leads WHERE owner_id=$1 GROUP BY status`)
			resp["tiers"] = groupCount(`SELECT COALESCE(tier,'?'), count(*) FROM leads WHERE owner_id=$1 GROUP BY 1`)

			leads := []map[string]any{}
			if rows, err := db.QueryContext(ctx, `
				SELECT l.id, COALESCE(l.score,0), COALESCE(l.tier,''), l.status, c.name,
				       EXISTS(SELECT 1 FROM contacts ct WHERE ct.company_id=c.id AND ct.owner_id=$1 AND COALESCE(ct.email,'')<>'') AS has_email
				FROM leads l JOIN companies c ON c.id=l.company_id WHERE l.owner_id=$1 ORDER BY l.score DESC LIMIT 20`, ownerID); err == nil {
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
				WHERE o.owner_id=$1 AND NOT o.approved AND o.outcome IS NULL ORDER BY o.id`, ownerID); err == nil {
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
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		name := strings.ToLower(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "need ?name", http.StatusBadRequest)
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		missions := []map[string]string{}
		if rows, err := db.QueryContext(r.Context(), `
			SELECT to_char(created_at,'DD Mon HH24:MI'), COALESCE(input,''), COALESCE(decision,''),
			       COALESCE(result,''), COALESCE(lesson,'')
			FROM experiences WHERE owner_id=$1 AND agent=$2 ORDER BY created_at DESC LIMIT 10`, ownerID, name); err == nil {
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
		db.QueryRowContext(r.Context(), `SELECT count(*) FROM experiences WHERE owner_id=$1 AND agent=$2`, ownerID, name).Scan(&count)
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
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		var req struct {
			Niche string `json:"niche"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Niche == "" {
			http.Error(w, "need {niche}", http.StatusBadRequest)
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		runAgent(&Oracle{Brain: brain, DB: db, OwnerID: ownerID}, req.Niche, "🔎 ORACLE hunting: "+req.Niche)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /api/atlas", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		var req struct {
			Lead int64 `json:"lead"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Lead == 0 {
			http.Error(w, "need {lead}", http.StatusBadRequest)
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		runAgent(&Atlas{Brain: brain, DB: db, OwnerID: ownerID}, fmt.Sprint(req.Lead), fmt.Sprintf("✍️ ATLAS drafting for lead %d", req.Lead))
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /api/pixel", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		var req struct {
			Lead int64 `json:"lead"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.Lead == 0 {
			http.Error(w, "need {lead}", http.StatusBadRequest)
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		act.add("🎨 PIXEL reviewing lead %d", req.Lead)
		busy.set("PIXEL", true)
		critique, err := RunPixel(r.Context(), db, ownerID, brain, req.Lead)
		busy.set("PIXEL", false)
		if err != nil {
			act.add("✗ PIXEL: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		act.add("✓ PIXEL: design critique for lead %d", req.Lead)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"critique": critique})
	})

	mux.HandleFunc("POST /api/inbox", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		act.add("📬 HERALD checking inbox for replies")
		busy.set("HERALD", true)
		go func() {
			defer busy.set("HERALD", false)
			n, err := CheckInbox(context.Background(), db, ownerID)
			if err != nil {
				act.add("✗ inbox: %v", err)
				return
			}
			act.add("✓ inbox: %d replies processed", n)
		}()
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("POST /api/brief", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		act.add("📨 briefing requested")
		busy.set("FRIDAY", true)
		busy.set("CALEB", true)
		go func() {
			defer busy.set("FRIDAY", false)
			defer busy.set("CALEB", false)
			if err := RunBrief(context.Background(), db, ownerID, brain); err != nil {
				act.add("✗ brief: %v", err)
			} else {
				act.add("✓ brief delivered to Telegram")
			}
		}()
		w.WriteHeader(http.StatusAccepted)
	})

	// HERALD dispatch — only works on drafts Sobhan already approved.
	mux.HandleFunc("POST /api/send", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		var req struct {
			ID int64 `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == 0 {
			http.Error(w, "need {id}", http.StatusBadRequest)
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		to, err := HeraldSend(r.Context(), db, ownerID, req.ID)
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
		if _, ok := requireAdmin(w, r, db); !ok {
			return
		}
		var req struct {
			ID int64 `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == 0 {
			http.Error(w, "need {id}", http.StatusBadRequest)
			return
		}
		ownerID, err := defaultOwnerID(r.Context(), db)
		if err != nil {
			http.Error(w, "owner lookup failed", http.StatusInternalServerError)
			return
		}
		draft, err := ApproveOutreach(r.Context(), db, ownerID, req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		act.add("✅ outreach #%d approved by Sobhan", req.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"draft": draft})
	})
}
