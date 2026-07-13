package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/webhook"
)

var billingDB *sql.DB

type stripeCheckoutSession struct {
	ClientReferenceID string            `json:"client_reference_id"`
	Customer          string            `json:"customer"`
	Metadata          map[string]string `json:"metadata"`
	Subscription      string            `json:"subscription"`
}

type stripeSubscription struct {
	ID               string `json:"id"`
	Customer         string `json:"customer"`
	CurrentPeriodEnd int64  `json:"current_period_end"`
	Status           string `json:"status"`
	Items            struct {
		Data []struct {
			Price struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

type stripeInvoice struct {
	ID               string `json:"id"`
	Customer         string `json:"customer"`
	AmountDue        int64  `json:"amount_due"`
	AmountPaid       int64  `json:"amount_paid"`
	Currency         string `json:"currency"`
	HostedInvoiceURL string `json:"hosted_invoice_url"`
	Status           string `json:"status"`
}

// HandleStripeWebhook verifies Stripe's signature before syncing billing state.
func HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		log.Printf("Stripe webhook ignored: STRIPE_WEBHOOK_SECRET is not configured")
		w.WriteHeader(http.StatusOK)
		return
	}
	if billingDB == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	event, err := webhook.ConstructEvent(body, r.Header.Get("Stripe-Signature"), secret)
	if err != nil {
		http.Error(w, "invalid Stripe signature", http.StatusBadRequest)
		return
	}
	if err := handleStripeEvent(r.Context(), billingDB, event); err != nil {
		log.Printf("Stripe webhook %s: %v", event.Type, err)
		http.Error(w, "could not sync billing event", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleStripeEvent(ctx context.Context, db *sql.DB, event stripe.Event) error {
	switch event.Type {
	case "checkout.session.completed":
		var session stripeCheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return fmt.Errorf("parse checkout session: %w", err)
		}
		return syncCheckoutSession(ctx, db, session)
	case "customer.subscription.updated", "customer.subscription.deleted":
		var subscription stripeSubscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			return fmt.Errorf("parse subscription: %w", err)
		}
		if event.Type == "customer.subscription.deleted" {
			subscription.Status = "canceled"
		}
		return syncSubscription(ctx, db, subscription)
	case "invoice.paid", "invoice.payment_failed":
		var invoice stripeInvoice
		if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
			return fmt.Errorf("parse invoice: %w", err)
		}
		if event.Type == "invoice.paid" {
			invoice.Status = "paid"
		} else if invoice.Status == "" {
			invoice.Status = "open"
		}
		return syncInvoice(ctx, db, invoice)
	default:
		return nil
	}
}

func syncCheckoutSession(ctx context.Context, db *sql.DB, session stripeCheckoutSession) error {
	if session.ClientReferenceID == "" || session.Customer == "" {
		log.Printf("Stripe checkout session missing client reference or customer")
		return nil
	}
	userID, err := strconv.ParseInt(session.ClientReferenceID, 10, 64)
	if err != nil || userID < 1 {
		log.Printf("Stripe checkout session has invalid client reference ID %q", session.ClientReferenceID)
		return nil
	}
	result, err := db.ExecContext(ctx, `UPDATE users SET stripe_customer_id = $1 WHERE id = $2`, session.Customer, userID)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		log.Printf("Stripe checkout session references missing user %d", userID)
		return nil
	}

	if session.Subscription == "" {
		return nil
	}
	plan := session.Metadata["plan"]
	if plan == "" {
		plan = planForPrice(session.Metadata["price"])
	}
	return upsertSubscription(ctx, db, &userID, session.Subscription, plan, "active", nil)
}

func syncSubscription(ctx context.Context, db *sql.DB, subscription stripeSubscription) error {
	if subscription.ID == "" {
		return errors.New("subscription ID is missing")
	}
	userID, err := userIDForStripeCustomer(ctx, db, subscription.Customer)
	if err != nil {
		return err
	}
	var periodEnd *time.Time
	if subscription.CurrentPeriodEnd > 0 {
		end := time.Unix(subscription.CurrentPeriodEnd, 0)
		periodEnd = &end
	}
	plan := ""
	if len(subscription.Items.Data) > 0 {
		plan = planForPrice(subscription.Items.Data[0].Price.ID)
	}
	return upsertSubscription(ctx, db, userID, subscription.ID, plan, subscription.Status, periodEnd)
}

func syncInvoice(ctx context.Context, db *sql.DB, invoice stripeInvoice) error {
	if invoice.ID == "" {
		return errors.New("invoice ID is missing")
	}
	userID, err := userIDForStripeCustomer(ctx, db, invoice.Customer)
	if err != nil {
		return err
	}
	amount := invoice.AmountPaid
	if amount == 0 {
		amount = invoice.AmountDue
	}
	return upsertInvoice(ctx, db, userID, invoice.ID, amount, invoice.Currency, invoice.Status, invoice.HostedInvoiceURL)
}

func userIDForStripeCustomer(ctx context.Context, db *sql.DB, customerID string) (*int64, error) {
	if customerID == "" {
		return nil, nil
	}
	var userID int64
	err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE stripe_customer_id = $1`, customerID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &userID, nil
}

func upsertSubscription(ctx context.Context, db *sql.DB, userID *int64, stripeSubID, plan, status string, periodEnd *time.Time) error {
	var id any
	if userID != nil {
		id = *userID
	}
	var end any
	if periodEnd != nil {
		end = *periodEnd
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO subscriptions (user_id, stripe_sub_id, plan, status, current_period_end)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (stripe_sub_id) DO UPDATE
		SET user_id = COALESCE(EXCLUDED.user_id, subscriptions.user_id),
		    plan = COALESCE(NULLIF(EXCLUDED.plan, ''), subscriptions.plan),
		    status = EXCLUDED.status,
		    current_period_end = COALESCE(EXCLUDED.current_period_end, subscriptions.current_period_end),
		    updated_at = now()`, id, stripeSubID, plan, status, end)
	return err
}

func upsertInvoice(ctx context.Context, db *sql.DB, userID *int64, stripeInvoiceID string, amount int64, currency, status, hostedURL string) error {
	var id any
	if userID != nil {
		id = *userID
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO invoices (user_id, stripe_invoice_id, amount_cents, currency, status, hosted_url)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (stripe_invoice_id) DO UPDATE
		SET user_id = COALESCE(EXCLUDED.user_id, invoices.user_id),
		    amount_cents = EXCLUDED.amount_cents,
		    currency = EXCLUDED.currency,
		    status = EXCLUDED.status,
		    hosted_url = EXCLUDED.hosted_url`, id, stripeInvoiceID, amount, currency, status, hostedURL)
	return err
}
