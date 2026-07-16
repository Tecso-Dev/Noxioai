package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PIXEL — design & motion critic persona: Emil Kowalski-school restraint,
// purposeful motion, micro-interactions, taste over flash. Appears via `jarvis pixel`.

const pixelPrompt = `You are PIXEL, NOXIOAI's design intelligence. You think in the school of Emil Kowalski: restraint, purposeful motion, micro-interactions, taste over flash. You advise Sobhan Azimzadeh — solo software/design agency founder.

Write a short design critique of this company's CURRENT website, grounded only in the lead notes and observed problem:
DESIGN DIAGNOSIS — 2-3 sentences on what is visually or UX-wise weak.
3 FIXES — three specific, concrete design improvements.
ONE MOTION IDEA — a single tasteful micro-interaction.

Max 160 words. Plain text only. This becomes ammunition for outreach.

Company:
Name: %s
Website: %s
Industry: %s
Notes: %s
Observed problem: %s`

func RunPixel(ctx context.Context, db *sql.DB, ownerID int64, brain *Brain, leadID int64) (string, error) {
	lead, err := GetLead(ctx, db, ownerID, leadID)
	if err != nil {
		return "", err
	}
	critique, err := brain.Chat([]Message{{Role: "user",
		Content: fmt.Sprintf(pixelPrompt, lead.Name, lead.Website, lead.Industry, lead.Notes, lead.ObservedProblem)}}, nil)
	if err != nil {
		return "", err
	}
	critique = strings.TrimSpace(critique)
	AddExperience(ctx, db, ownerID, "pixel", fmt.Sprintf("design review: %s", lead.Name), "wrote design critique", "delivered", oneLine(critique, 120))
	return critique, nil
}
