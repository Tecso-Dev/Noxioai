package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/mail"
	"strings"
)

const waitlistServicesURL = "https://noxioai.com/services"

func isValidWaitlistEmail(value string) bool {
	email := strings.TrimSpace(value)
	if email == "" {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

func writeWaitlistOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// registerWaitlist wires the public NOXIOAI product waitlist onto the mux.
func registerWaitlist(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/waitlist", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email   string `json:"email"`
			Name    string `json:"name"`
			Locale  string `json:"locale"`
			Source  string `json:"source"`
			Company string `json:"company"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Bots get an indistinguishable success response without touching the DB.
		if strings.TrimSpace(req.Company) != "" {
			writeWaitlistOK(w)
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		if !isValidWaitlistEmail(email) {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}

		name := strings.TrimSpace(req.Name)
		locale := strings.TrimSpace(req.Locale)
		if locale == "" {
			locale = "en"
		}
		source := strings.TrimSpace(req.Source)
		result, err := db.ExecContext(r.Context(), `
			INSERT INTO waitlist (email, name, locale, source)
			VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''))
			ON CONFLICT (email) DO NOTHING`, email, name, locale, source)
		if err != nil {
			log.Println("waitlist: insert:", err)
			http.Error(w, "could not join waitlist", http.StatusInternalServerError)
			return
		}
		rows, err := result.RowsAffected()
		if err != nil {
			log.Println("waitlist: rows affected:", err)
			http.Error(w, "could not join waitlist", http.StatusInternalServerError)
			return
		}

		if rows > 0 {
			go func(to, name, locale string) {
				if err := sendWelcomeEmail(to, name, locale); err != nil {
					log.Println("waitlist: send welcome email:", err)
				}
			}(email, name, locale)
		}

		// Duplicate and new signups deliberately receive the same response.
		writeWaitlistOK(w)
	})
}

func sendWelcomeEmail(to, name, locale string) error {
	// The locale is captured with the signup so this copy can be localized later.
	_ = locale
	greeting := "Hi,"
	if name = strings.TrimSpace(name); name != "" {
		greeting = fmt.Sprintf("Hi %s,", name)
	}

	const subject = "Welcome to the NOXIOAI waitlist"
	text := fmt.Sprintf(`%s

Thanks for joining the NOXIOAI waitlist.

NOXIOAI builds AI employees that work while you sleep, helping businesses keep important work moving around the clock.

We'll be in touch with early access opportunities and product updates. In the meantime, explore what we're building:
%s

Questions? Reply to hi@noxioai.com.

— NOXIOAI`, greeting, waitlistServicesURL)

	htmlGreeting := "Hi,"
	if name != "" {
		htmlGreeting = fmt.Sprintf("Hi %s,", html.EscapeString(name))
	}
	body := fmt.Sprintf(`%s<br><br>Thanks for joining the NOXIOAI waitlist.<br><br>NOXIOAI builds AI employees that work while you sleep, helping businesses keep important work moving around the clock.<br><br>We'll be in touch with early access opportunities and product updates.`, htmlGreeting)
	htmlBody := authMailHTML("Welcome to the NOXIOAI waitlist", body, waitlistServicesURL, "Explore NOXIOAI services")
	return deliverMail(to, subject, text, htmlBody)
}
