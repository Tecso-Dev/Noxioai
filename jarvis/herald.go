package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/smtp"
	"os"
	"strings"
)

// HERALD (SPEC §8, email channel): dispatches APPROVED email outreach via
// Gmail SMTP. Principle 1 holds in code — it refuses anything unapproved.

func splitDraft(draft string) (subject, body string) {
	rest, ok := strings.CutPrefix(draft, "Subject: ")
	if !ok {
		return "", strings.TrimSpace(draft)
	}
	subject, body, _ = strings.Cut(rest, "\n\n")
	return strings.TrimSpace(subject), strings.TrimSpace(body)
}

// HeraldSend emails an approved outreach draft to the lead's first known
// contact address. Returns the recipient on success.
func HeraldSend(ctx context.Context, db *sql.DB, outreachID int64) (string, error) {
	user := os.Getenv("JARVIS_SMTP_USER")
	pass := os.Getenv("JARVIS_SMTP_PASS")
	if user == "" || pass == "" {
		return "", fmt.Errorf("HERALD offline: set JARVIS_SMTP_PASS in jarvis/.env (Google account → App passwords, 2-minute errand)")
	}

	var draft, channel, company string
	var approved bool
	var companyID int64
	var outcome sql.NullString
	var sentAt sql.NullTime
	err := db.QueryRowContext(ctx, `
		SELECT o.draft, o.channel, o.approved, c.id, c.name, o.outcome, o.sent_at
		FROM outreach o JOIN leads l ON l.id=o.lead_id JOIN companies c ON c.id=l.company_id
		WHERE o.id=$1`, outreachID).Scan(&draft, &channel, &approved, &companyID, &company, &outcome, &sentAt)
	if err != nil {
		return "", fmt.Errorf("outreach #%d: %w", outreachID, err)
	}
	if outcome.Valid || sentAt.Valid {
		return "", fmt.Errorf("outreach #%d already sent", outreachID)
	}
	if !approved {
		return "", fmt.Errorf("outreach #%d is NOT approved — Principle 1: approve it first", outreachID)
	}
	if channel != "email" {
		return "", fmt.Errorf("outreach #%d is %s — HERALD only dispatches email; send that one manually", outreachID, channel)
	}

	var to string
	err = db.QueryRowContext(ctx, `
		SELECT email FROM contacts WHERE company_id=$1 AND COALESCE(email,'')<>'' ORDER BY id LIMIT 1`,
		companyID).Scan(&to)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("no contact email on file for %s — send manually or add a contact", company)
	}
	if err != nil {
		return "", err
	}

	subject, body := splitDraft(draft)
	if subject == "" {
		return "", fmt.Errorf("draft #%d has no Subject line", outreachID)
	}
	msg := fmt.Sprintf("From: Sobhan Azimzadeh — NOXIOAI <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		user, to, subject, body)
	auth := smtp.PlainAuth("", user, pass, "smtp.gmail.com")
	if err := smtp.SendMail("smtp.gmail.com:587", auth, user, []string{to}, []byte(msg)); err != nil {
		return "", fmt.Errorf("smtp: %w", err)
	}

	if err := SetOutcome(ctx, db, outreachID, "sent"); err != nil {
		return to, err
	}
	return to, AddExperience(ctx, db, "herald",
		fmt.Sprintf("outreach %d → %s (%s)", outreachID, to, company),
		"dispatched approved email", "sent", "")
}
