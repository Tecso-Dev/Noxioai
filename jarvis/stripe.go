package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/stripe/stripe-go/v80"
	billingportalsession "github.com/stripe/stripe-go/v80/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v80/checkout/session"
)

func init() {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
}

func priceForPlan(plan string) string {
	switch plan {
	case "starter":
		return os.Getenv("STRIPE_PRICE_STARTER_FOUNDER")
	case "pro":
		return os.Getenv("STRIPE_PRICE_PRO_FOUNDER")
	case "agency":
		return os.Getenv("STRIPE_PRICE_AGENCY_FOUNDER")
	default:
		return ""
	}
}

func planForPrice(price string) string {
	for _, plan := range []string{"starter", "pro", "agency"} {
		if price != "" && price == priceForPlan(plan) {
			return plan
		}
	}
	return ""
}

func appBaseURL() string {
	return strings.TrimRight(envOr("APP_BASE_URL", "http://localhost:3000"), "/")
}

// CheckoutSession creates a hosted Stripe Checkout session for a subscription.
func CheckoutSession(ctx context.Context, user *User, plan string) (url string, err error) {
	if user == nil {
		return "", errors.New("user is required")
	}
	price := priceForPlan(plan)
	if price == "" {
		return "", errors.New("unknown plan")
	}
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if stripe.Key == "" {
		return "", errors.New("STRIPE_SECRET_KEY is not configured")
	}

	params := &stripe.CheckoutSessionParams{
		Params:            stripe.Params{Context: ctx},
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		ClientReferenceID: stripe.String(strconv.FormatInt(user.ID, 10)),
		SuccessURL:        stripe.String(appBaseURL() + "/app/billing?success=1"),
		CancelURL:         stripe.String(appBaseURL() + "/#pricing"),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Price:    stripe.String(price),
			Quantity: stripe.Int64(1),
		}},
	}
	params.AddMetadata("plan", plan)
	params.AddMetadata("price", price)
	if user.StripeCustomerID != "" {
		params.Customer = stripe.String(user.StripeCustomerID)
	} else {
		params.CustomerEmail = stripe.String(user.Email)
	}

	session, err := checkoutsession.New(params)
	if err != nil {
		return "", err
	}
	return session.URL, nil
}

// PortalSession creates a hosted Stripe Billing Portal session.
func PortalSession(ctx context.Context, customerID string) (url string, err error) {
	if customerID == "" {
		return "", errors.New("customer ID is required")
	}
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if stripe.Key == "" {
		return "", errors.New("STRIPE_SECRET_KEY is not configured")
	}

	params := &stripe.BillingPortalSessionParams{
		Params:    stripe.Params{Context: ctx},
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(appBaseURL() + "/app"),
	}
	session, err := billingportalsession.New(params)
	if err != nil {
		return "", err
	}
	return session.URL, nil
}

func registerBilling(mux *http.ServeMux, db *sql.DB) {
	billingDB = db

	mux.HandleFunc("POST /api/billing/checkout", func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Plan string `json:"plan"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || priceForPlan(req.Plan) == "" {
			http.Error(w, "invalid plan", http.StatusBadRequest)
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
		url, err := CheckoutSession(r.Context(), user, req.Plan)
		if err != nil {
			http.Error(w, "could not create checkout session", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": url})
	})

	mux.HandleFunc("POST /api/billing/portal", func(w http.ResponseWriter, r *http.Request) {
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
		if user.StripeCustomerID == "" {
			http.Error(w, "billing profile unavailable", http.StatusBadRequest)
			return
		}
		url, err := PortalSession(r.Context(), user.StripeCustomerID)
		if err != nil {
			http.Error(w, "could not create portal session", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": url})
	})

	mux.HandleFunc("POST /api/stripe/webhook", HandleStripeWebhook)
}
