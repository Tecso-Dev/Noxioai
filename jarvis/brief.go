package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// FRIDAY morning briefing (SPEC Phase 4). Not an agent — one function on a
// timer (launchd, 08:00). Reads the CRM, composes the day, pings Telegram.
func RunBrief(ctx context.Context, db *sql.DB, brain *Brain) error {
	var b strings.Builder
	fmt.Fprintf(&b, "⚡ JARVIS morning brief — %s\n", time.Now().Format("Mon 2 Jan, 15:04"))

	var total, new24 int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*), count(*) FILTER (WHERE created_at > now() - interval '24 hours') FROM leads`).
		Scan(&total, &new24); err != nil {
		return err
	}
	fmt.Fprintf(&b, "\n📊 Leads: %d total, %d new since yesterday\n", total, new24)

	rows, err := db.QueryContext(ctx, `
		SELECT l.id, COALESCE(l.score,0), c.name
		FROM leads l JOIN companies c ON c.id = l.company_id
		WHERE l.status = 'new' ORDER BY l.score DESC LIMIT 3`)
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
		WHERE NOT o.approved AND o.outcome IS NULL ORDER BY o.id LIMIT 5`)
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
		WHERE o.sent_at > now() - interval '24 hours' AND o.outcome IS NOT NULL`)
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

	// One recommendation — best effort; the brief still goes out if the brain is down.
	if rec, err := brain.Chat([]Message{{Role: "user", Content: "You are JARVIS, Sobhan's business AI (address him as Sir, one short sentence, concrete). Given this status, what is THE one action for today?\n\n" + b.String()}}, nil); err == nil {
		if r := oneLine(rec, 200); r != "" {
			b.WriteString("\n💡 " + r + "\n")
		}
	}

	return SendTelegram(b.String())
}
