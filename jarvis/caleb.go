package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// CALEB — marketing strategist persona (SPEC §8): Caleb Ralston-school brand
// thinking. No infrastructure — a persona prompt over live CRM numbers.
// Appears in the morning briefing and via `jarvis caleb`.

const calebPrompt = `You are CALEB, NOXIOAI's marketing strategist. You think in the school of Caleb Ralston's published work: brand-first, long-game, audience before ads, specificity over hype, one channel done well beats five done poorly. You advise Sobhan Azimzadeh — solo software-agency founder (web development, e-commerce, AI automation, UI/UX). His edge: proposal craft. His bottleneck: sales pipeline.

Given today's pipeline, write his morning marketing memo:
WHAT THE NUMBERS SAY — one blunt paragraph, no soft language.
TODAY'S 3 MOVES — three concrete actions doable today, most leverage first, imperatives only.
CONTENT ANGLE — one specific post idea for today grounded in what the pipeline shows.

Max 180 words. Plain text only.

Pipeline:
%s`

func crmSnapshot(ctx context.Context, db *sql.DB) string {
	var b strings.Builder
	var total, new24, pending, approved, sent int
	db.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE created_at > now()-interval '24 hours') FROM leads`).Scan(&total, &new24)
	db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE NOT approved AND outcome IS NULL),
		count(*) FILTER (WHERE approved), count(*) FILTER (WHERE sent_at IS NOT NULL) FROM outreach`).Scan(&pending, &approved, &sent)
	fmt.Fprintf(&b, "Leads: %d total, %d new in 24h. Outreach: %d approved, %d sent, %d awaiting approval.\n", total, new24, approved, sent, pending)

	if rows, err := db.QueryContext(ctx, `SELECT status, count(*) FROM leads GROUP BY status ORDER BY 2 DESC`); err == nil {
		b.WriteString("Funnel: ")
		for rows.Next() {
			var s string
			var n int
			if rows.Scan(&s, &n) == nil {
				fmt.Fprintf(&b, "%s=%d ", s, n)
			}
		}
		rows.Close()
		b.WriteString("\n")
	}
	if rows, err := db.QueryContext(ctx, `
		SELECT c.name, COALESCE(l.score,0), COALESCE(l.observed_problem,'')
		FROM leads l JOIN companies c ON c.id=l.company_id
		WHERE l.status='new' ORDER BY l.score DESC LIMIT 3`); err == nil {
		b.WriteString("Top unworked leads:\n")
		for rows.Next() {
			var name, prob string
			var score int
			if rows.Scan(&name, &score, &prob) == nil {
				fmt.Fprintf(&b, "- %s (%d): %s\n", name, score, oneLine(prob, 90))
			}
		}
		rows.Close()
	}
	return b.String()
}

func RunCaleb(ctx context.Context, db *sql.DB, brain *Brain) (string, error) {
	memo, err := brain.Chat([]Message{{Role: "user",
		Content: fmt.Sprintf(calebPrompt, crmSnapshot(ctx, db))}}, nil)
	if err != nil {
		return "", err
	}
	memo = strings.TrimSpace(memo)
	AddExperience(ctx, db, "caleb", "morning pipeline read", "wrote marketing memo", "delivered", oneLine(memo, 120))
	return memo, nil
}
