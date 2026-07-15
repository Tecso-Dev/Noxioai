package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"time"
)

// deliverMail is the single email transport for the whole app.
// Resend HTTPS API when RESEND_API_KEY is set (production: aeza blocks outbound
// SMTP ports entirely), Gmail SMTP fallback otherwise (dev/local).
func deliverMail(to, subject, text, html string) error {
	return deliverMailFrom(mailFrom(), to, subject, text, html)
}

// deliverMailFrom sends with an explicit From identity (must be on the
// verified domain), e.g. personal-touch outreach vs transactional hi@.
func deliverMailFrom(from, to, subject, text, html string) error {
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		return sendViaResend(key, from, to, subject, text, html)
	}
	return sendViaSMTP(from, to, subject, text, html)
}

func mailFrom() string {
	if f := os.Getenv("JARVIS_MAIL_FROM"); f != "" {
		return f
	}
	if u := os.Getenv("JARVIS_SMTP_USER"); u != "" {
		return "NOXIOAI <" + u + ">"
	}
	return "NOXIOAI <hi@noxioai.com>"
}

func sendViaResend(key, from, to, subject, text, html string) error {
	payload := map[string]any{"from": from, "to": []string{to}, "subject": subject}
	// until inbound routing for the domain exists, replies land in the business inbox
	if rt := os.Getenv("JARVIS_REPLY_TO"); rt != "" {
		payload["reply_to"] = rt
	}
	if text != "" {
		payload["text"] = text
	}
	if html != "" {
		payload["html"] = html
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend: %s: %s", resp.Status, detail)
	}
	return nil
}

func sendViaSMTP(from, to, subject, text, html string) error {
	user := os.Getenv("JARVIS_SMTP_USER")
	pass := os.Getenv("JARVIS_SMTP_PASS")
	if user == "" || pass == "" {
		return fmt.Errorf("no mail transport configured: set RESEND_API_KEY or JARVIS_SMTP_USER/JARVIS_SMTP_PASS")
	}
	var msg string
	if html == "" {
		msg = fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
			from, to, subject, text)
	} else {
		const boundary = "noxioai-mail-boundary"
		msg = fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%s\r\n\r\n"+
				"--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n"+
				"--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n--%s--\r\n",
			from, to, subject, boundary, boundary, text, boundary, html, boundary)
	}
	auth := smtp.PlainAuth("", user, pass, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, user, []string{to}, []byte(msg))
}
