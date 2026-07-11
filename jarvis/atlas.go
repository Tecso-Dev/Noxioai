package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ATLAS — outreach agent (SPEC Phase 3): read a lead, draft personalized
// email + LinkedIn outreach, store UNAPPROVED. Principle 1: nothing sends
// without `jarvis approve`; sending itself stays manual copy/paste in v1.
type Atlas struct {
	Brain *Brain
	DB    *sql.DB
}

func (a *Atlas) Name() string { return "atlas" }

type draft struct {
	Email struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	} `json:"email"`
	Linkedin string `json:"linkedin"`
}

const draftPrompt = `You are ATLAS, the outreach agent of NOXIOAI — a software agency (web development, e-commerce, AI automation, UI/UX design) run by Sobhan Azimzadeh.

Write first-contact outreach to this company. NEVER generic: cite the company by name, name the observed problem concretely, propose the solution, and state the business value for THEM. Plain language, direct, expert, zero buzzwords, no flattery. Email 120-180 words. LinkedIn message 50-80 words. Sign "Sobhan — NOXIOAI".

Reply with ONLY this JSON: {"email":{"subject":"","body":""},"linkedin":""}

Company: %s (%s)
Industry: %s
About: %s
Lead score: %d (%s) — %s
Observed problem: %s
Suggested offer: %s
%s`

func (a *Atlas) Run(ctx context.Context, task Task) (Result, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(task.Input), 10, 64)
	if err != nil {
		return Result{}, fmt.Errorf("usage: jarvis atlas <lead-id>  (list ids with: jarvis leads)")
	}
	lead, err := GetLead(ctx, a.DB, id)
	if err != nil {
		return Result{}, fmt.Errorf("lead %d: %w", id, err)
	}

	lessonsBlock := ""
	if lessons, _ := RecentLessons(ctx, a.DB, "atlas", 3); len(lessons) > 0 {
		lessonsBlock = "Lessons from past outreach:\n- " + strings.Join(lessons, "\n- ")
	}
	prompt := fmt.Sprintf(draftPrompt, lead.Name, lead.Website, lead.Industry, lead.Notes,
		lead.Score, lead.Tier, lead.Reasoning, lead.ObservedProblem, lead.SuggestedOffer, lessonsBlock)

	out, err := a.Brain.Chat([]Message{{Role: "user", Content: prompt}}, nil)
	if err != nil {
		return Result{}, err
	}
	d, perr := parseDraftJSON(out, lead.Name)
	if perr != nil { // one retry — models decorate JSON or drop the company name
		out, err = a.Brain.Chat([]Message{
			{Role: "user", Content: prompt},
			{Role: "assistant", Content: out},
			{Role: "user", Content: "Invalid: " + perr.Error() + ". Reply again with ONLY the valid JSON, following every rule."},
		}, nil)
		if err != nil {
			return Result{}, err
		}
		if d, perr = parseDraftJSON(out, lead.Name); perr != nil {
			return Result{}, perr
		}
	}

	emailID, err := CreateOutreach(ctx, a.DB, lead.ID, "email", "Subject: "+d.Email.Subject+"\n\n"+d.Email.Body)
	if err != nil {
		return Result{}, err
	}
	liID, err := CreateOutreach(ctx, a.DB, lead.ID, "linkedin", d.Linkedin)
	if err != nil {
		return Result{}, err
	}
	if err := AddExperience(ctx, a.DB, "atlas",
		fmt.Sprintf("lead %d: %s (%s)", lead.ID, lead.Name, lead.Website),
		"drafted email + linkedin", "stored unapproved", lead.ObservedProblem); err != nil {
		return Result{}, err
	}

	fmt.Printf("\n── email draft #%d ─────────────────────────────\nSubject: %s\n\n%s\n", emailID, d.Email.Subject, d.Email.Body)
	fmt.Printf("\n── linkedin draft #%d ──────────────────────────\n%s\n", liID, d.Linkedin)
	return Result{Output: fmt.Sprintf("2 drafts stored UNAPPROVED for %s — review then `jarvis approve %d` / `jarvis approve %d`",
		lead.Name, emailID, liID)}, nil
}

func parseDraftJSON(out, company string) (*draft, error) {
	start := strings.Index(out, "{")
	end := strings.LastIndex(out, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object in model output")
	}
	var d draft
	if err := json.Unmarshal([]byte(out[start:end+1]), &d); err != nil {
		return nil, err
	}
	if d.Email.Subject == "" || len(d.Email.Body) < 100 || len(d.Linkedin) < 40 {
		return nil, fmt.Errorf("draft too thin (need subject, email ≥100 chars, linkedin ≥40 chars)")
	}
	// Principle 3 floor: the company must be named in the copy.
	token := strings.ToLower(strings.Fields(company)[0])
	if len(token) >= 3 && !strings.Contains(strings.ToLower(d.Email.Body+d.Linkedin), token) {
		return nil, fmt.Errorf("draft does not mention the company %q", company)
	}
	return &d, nil
}
