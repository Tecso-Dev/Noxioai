package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// FRIDAY morning briefing (SPEC Phase 4). Not an agent — one function on a
// timer (launchd, 08:00). Reads the CRM, composes the day, pings Telegram.
func RunBrief(ctx context.Context, db *sql.DB, ownerID int64, brain *Brain) error {
	if _, err := CheckInbox(ctx, db, ownerID); err != nil {
		// Inbox replies are supplemental; the brief must still be delivered.
	}

	var b strings.Builder
	fmt.Fprintf(&b, "⚡ JARVIS morning brief — %s\n", time.Now().Format("Mon 2 Jan, 15:04"))

	var total, new24 int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*), count(*) FILTER (WHERE created_at > now() - interval '24 hours') FROM leads WHERE owner_id=$1`, ownerID).
		Scan(&total, &new24); err != nil {
		return err
	}
	fmt.Fprintf(&b, "\n📊 Leads: %d total, %d new since yesterday\n", total, new24)

	rows, err := db.QueryContext(ctx, `
		SELECT l.id, COALESCE(l.score,0), c.name
		FROM leads l JOIN companies c ON c.id = l.company_id
		WHERE l.owner_id = $1 AND l.status = 'new' ORDER BY l.score DESC LIMIT 3`, ownerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	top := ""
	for rows.Next() {
		var id int64
		var score int
		var name string
		if err := rows.Scan(&id, &score, &name); err != nil {
			return err
		}
		top += fmt.Sprintf("  #%d %s (%d) — jarvis atlas %d\n", id, name, score, id)
	}
	if top != "" {
		b.WriteString("\n🎯 Top unworked leads:\n" + top)
	}

	drafts, err := db.QueryContext(ctx, `
		SELECT o.id, o.channel, c.name
		FROM outreach o JOIN leads l ON l.id = o.lead_id JOIN companies c ON c.id = l.company_id
		WHERE o.owner_id = $1 AND NOT o.approved AND o.outcome IS NULL ORDER BY o.id LIMIT 5`, ownerID)
	if err != nil {
		return err
	}
	defer drafts.Close()
	pending := ""
	for drafts.Next() {
		var id int64
		var channel, name string
		if err := drafts.Scan(&id, &channel, &name); err != nil {
			return err
		}
		pending += fmt.Sprintf("  #%d %s → %s — jarvis approve %d\n", id, channel, name, id)
	}
	if pending != "" {
		b.WriteString("\n✍️ Drafts awaiting your approval:\n" + pending)
	}

	outcomes, err := db.QueryContext(ctx, `
		SELECT COALESCE(o.outcome,''), c.name
		FROM outreach o JOIN leads l ON l.id = o.lead_id JOIN companies c ON c.id = l.company_id
		WHERE o.owner_id = $1 AND l.updated_at > now() - interval '24 hours' AND o.outcome IS NOT NULL`, ownerID)
	if err != nil {
		return err
	}
	defer outcomes.Close()
	moved := ""
	for outcomes.Next() {
		var outcome, name string
		if err := outcomes.Scan(&outcome, &name); err != nil {
			return err
		}
		moved += fmt.Sprintf("  %s: %s\n", name, outcome)
	}
	if moved != "" {
		b.WriteString("\n📬 Movement since yesterday:\n" + moved)
	}

	// CALEB's marketing memo — best effort; the brief still goes out if the brain is down.
	if memo, err := RunCaleb(ctx, db, ownerID, brain); err == nil && memo != "" {
		b.WriteString("\n📈 CALEB — marketing memo:\n" + memo + "\n")
	}

	// Brain cost watch (user asked to monitor the DeepSeek balance).
	if bal := deepseekBalance(); bal != "" {
		b.WriteString("\n🧠 Brain balance: " + bal + "\n")
	}

	return SendTelegram(b.String())
}

// deepseekBalance returns the remaining API credit, or "" when the brain
// isn't DeepSeek or the check fails (never blocks the briefing).
func deepseekBalance() string {
	key := os.Getenv("JARVIS_API_KEY")
	if key == "" || !strings.Contains(os.Getenv("JARVIS_BASE_URL"), "deepseek") {
		return ""
	}
	req, err := http.NewRequest("GET", "https://api.deepseek.com/user/balance", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		BalanceInfos []struct {
			Currency     string `json:"currency"`
			TotalBalance string `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.BalanceInfos) == 0 {
		return ""
	}
	return "$" + out.BalanceInfos[0].TotalBalance + " " + out.BalanceInfos[0].Currency
}
