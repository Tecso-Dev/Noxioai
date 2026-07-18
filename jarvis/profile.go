package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type businessProfile struct {
	BusinessName  string `json:"business_name"`
	Sells         string `json:"sells"`
	IdealCustomer string `json:"ideal_customer"`
	City          string `json:"city"`
	Country       string `json:"country"`
	Language      string `json:"language"`
	Website       string `json:"website"`
	Telegram      string `json:"telegram"`
	Knowledge     string `json:"knowledge"`
	Goals         string `json:"goals"`
}

func normalizeBusinessProfile(profile businessProfile) (businessProfile, error) {
	profile.BusinessName = strings.TrimSpace(profile.BusinessName)
	profile.Sells = strings.TrimSpace(profile.Sells)
	profile.IdealCustomer = strings.TrimSpace(profile.IdealCustomer)
	profile.City = strings.TrimSpace(profile.City)
	profile.Country = strings.TrimSpace(profile.Country)
	profile.Language = strings.TrimSpace(profile.Language)
	profile.Website = strings.TrimSpace(profile.Website)
	profile.Telegram = strings.TrimSpace(profile.Telegram)
	profile.Knowledge = strings.TrimSpace(profile.Knowledge)
	profile.Goals = strings.TrimSpace(profile.Goals)
	if profile.BusinessName == "" || profile.Knowledge == "" {
		return businessProfile{}, errors.New("business_name and knowledge are required")
	}
	return profile, nil
}

func writeProfileJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

// registerProfile wires the tenant-owned business profile API onto the mux.
func registerProfile(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/profile", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil {
			http.Error(w, "could not get current user", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var profile businessProfile
		err = db.QueryRowContext(r.Context(), `
			SELECT COALESCE(business_name,''), COALESCE(sells,''),
			       COALESCE(ideal_customer,''), COALESCE(city,''),
			       COALESCE(country,''), COALESCE(language,''),
			       COALESCE(website,''), COALESCE(telegram,''),
			       COALESCE(knowledge,''), COALESCE(goals,'')
			FROM business_profiles WHERE owner_id = $1`, user.ID).
			Scan(&profile.BusinessName, &profile.Sells, &profile.IdealCustomer,
				&profile.City, &profile.Country, &profile.Language, &profile.Website,
				&profile.Telegram, &profile.Knowledge, &profile.Goals)
		if errors.Is(err, sql.ErrNoRows) {
			writeProfileJSON(w, map[string]any{})
			return
		}
		if err != nil {
			http.Error(w, "could not get profile", http.StatusInternalServerError)
			return
		}
		writeProfileJSON(w, profile)
	})

	mux.HandleFunc("POST /api/profile", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		user, err := currentUser(r.Context(), db, r)
		if err != nil {
			http.Error(w, "could not get current user", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var profile businessProfile
		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		profile, err = normalizeBusinessProfile(profile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err = db.ExecContext(r.Context(), `
			INSERT INTO business_profiles
				(owner_id, business_name, sells, ideal_customer, city, country,
				 language, website, telegram, knowledge, goals)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (owner_id) DO UPDATE SET
				business_name = EXCLUDED.business_name,
				sells = EXCLUDED.sells,
				ideal_customer = EXCLUDED.ideal_customer,
				city = EXCLUDED.city,
				country = EXCLUDED.country,
				language = EXCLUDED.language,
				website = EXCLUDED.website,
				telegram = EXCLUDED.telegram,
				knowledge = EXCLUDED.knowledge,
				goals = EXCLUDED.goals,
				updated_at = now()`,
			user.ID, profile.BusinessName, profile.Sells, profile.IdealCustomer,
			profile.City, profile.Country, profile.Language, profile.Website,
			profile.Telegram, profile.Knowledge, profile.Goals)
		if err != nil {
			http.Error(w, "could not save profile", http.StatusInternalServerError)
			return
		}
		writeProfileJSON(w, profile)
	})
}
