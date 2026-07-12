package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// CheckInbox looks for recent unread Gmail replies and advances their leads.
// It remains dormant until Gmail credentials are configured.
func CheckInbox(ctx context.Context, db *sql.DB) (replies int, err error) {
	user := os.Getenv("JARVIS_SMTP_USER")
	pass := os.Getenv("JARVIS_SMTP_PASS")
	if user == "" || pass == "" {
		return 0, nil
	}

	imapClient, err := client.DialTLS("imap.gmail.com:993", &tls.Config{ServerName: "imap.gmail.com"})
	if err != nil {
		return 0, fmt.Errorf("imap connect: %w", err)
	}
	defer imapClient.Logout()

	if err := imapClient.Login(user, pass); err != nil {
		return 0, fmt.Errorf("imap login: %w", err)
	}
	if _, err := imapClient.Select("INBOX", true); err != nil {
		return 0, fmt.Errorf("imap select INBOX: %w", err)
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	criteria.Since = time.Now().AddDate(0, 0, -7)
	messageIDs, err := imapClient.Search(criteria)
	if err != nil {
		return 0, fmt.Errorf("imap search unread mail: %w", err)
	}
	if len(messageIDs) == 0 {
		return 0, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(messageIDs...)
	messages := make(chan *imap.Message, 10)
	fetched := make(chan error, 1)
	// FETCH ENVELOPE is metadata-only, so it has BODY.PEEK semantics and never
	// marks an unread message as seen.
	go func() {
		fetched <- imapClient.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	var senders []string
	for message := range messages {
		if message.Envelope == nil {
			continue
		}
		for _, from := range message.Envelope.From {
			email := normalizeInboxEmail(from.MailboxName + "@" + from.HostName)
			if email != "" {
				senders = append(senders, email)
			}
		}
	}
	if err := <-fetched; err != nil {
		return 0, fmt.Errorf("imap fetch envelopes: %w", err)
	}

	for _, email := range senders {
		if err := ctx.Err(); err != nil {
			return replies, err
		}
		processed, err := processInboxReply(ctx, db, email)
		if err != nil {
			return replies, err
		}
		if processed {
			replies++
		}
	}
	return replies, nil
}

func normalizeInboxEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func processInboxReply(ctx context.Context, db *sql.DB, email string) (bool, error) {
	var outreachID int64
	var company string
	var outcome sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT o.id, c.name, o.outcome
		FROM contacts ct
		JOIN leads l ON l.company_id = ct.company_id
		JOIN companies c ON c.id = ct.company_id
		JOIN outreach o ON o.lead_id = l.id
		WHERE LOWER(ct.email) = LOWER($1) AND o.sent_at IS NOT NULL
		ORDER BY o.sent_at DESC, o.id DESC
		LIMIT 1`, email).Scan(&outreachID, &company, &outcome)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if outcome.Valid && outcome.String == "replied" {
		return false, nil
	}

	if err := SetOutcome(ctx, db, outreachID, "replied"); err != nil {
		return false, err
	}
	if err := AddExperience(ctx, db, "herald",
		"inbox: reply from "+email, "marked replied", "lead advanced", ""); err != nil {
		return false, err
	}
	if err := SendTelegram("📬 " + company + " replied to your outreach, Sir."); err != nil {
		return false, err
	}
	return true, nil
}
