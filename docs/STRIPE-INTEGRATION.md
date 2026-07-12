# NOXIOAI · Stripe Integration Plan

**Products: Billing + Invoicing · Mode: TEST · Account: acct_1TsJUv0SsjyDEeXC (PL)**

Companion to `PLATFORM-SPEC.md` §6. This is the concrete build plan for Phase 4.

## Status — what's done vs pending

| Item | State |
|---|---|
| Stripe account + test keys | ✅ stored in `jarvis/.env` (gitignored), verified |
| 3 subscription products + monthly prices (EUR) | ✅ created in test mode |
| Billing DB tables (users/sessions/subscriptions/invoices) | ✅ applied to Postgres |
| Go Stripe client + Checkout + Portal + webhook | ⬜ Phase 4 (needs auth first) |
| Nuxt pricing page + subscribe/manage UI | ⬜ Phase 4 |
| Account activation (real payments) | ⬜ Sobhan — submit business details in Stripe |

## Products (test mode — prices are placeholders, edit anytime)

| Plan | Price ID | € / mo |
|---|---|---|
| Starter | `price_1TsJf60SsjyDEeXCf59qN2KH` | 49 |
| Pro | `price_1TsJf80SsjyDEeXCGVfncQUw` | 149 |
| Agency | `price_1TsJfA0SsjyDEeXCYzud6l0Z` | 399 |

Price IDs live in `jarvis/.env` (`STRIPE_PRICE_*`). To reprice, create a new
price in Stripe and swap the ID — never edit an existing price's amount.

## Integration flow (best-practice, hosted-only)

```
1. User (logged in) clicks "Subscribe" on the pricing page
2. Nuxt → POST /api/billing/checkout {plan}
3. Go creates a Stripe Checkout Session (mode=subscription, the plan's price,
   client_reference_id = our user id, customer = user's stripe_customer_id or new)
   → returns the Checkout URL
4. Browser redirects to Stripe's HOSTED checkout (card entry happens THERE)
5. On success Stripe redirects back to /app/billing?success=1
6. Stripe fires webhooks → POST /api/stripe/webhook (signature-verified):
     checkout.session.completed      → link stripe_customer_id, create subscription row
     customer.subscription.updated   → update status / current_period_end
     customer.subscription.deleted   → status=canceled
     invoice.paid / invoice.finalized→ upsert invoices row (+ hosted_url)
7. Dashboard reads subscription/invoice rows — Stripe is the source of truth
8. "Manage / cancel" → POST /api/billing/portal → Stripe Billing Portal URL
```

## Security (non-negotiable — mirrors PLATFORM-SPEC §6)

- Card data only ever on Stripe's hosted pages. Never our forms, server, DB, or the assistant.
- Only Stripe IDs + statuses stored. No PAN, no CVC.
- Webhook handler verifies `Stripe-Signature` with `STRIPE_WEBHOOK_SECRET` before trusting any event; unverified → 400.
- Secret + webhook keys in `.env` only. Browser gets the **publishable** key alone.
- Idempotency: webhooks can retry/duplicate — upserts key on `stripe_*_id` (UNIQUE), never blind inserts.

## Go build checklist (Phase 4)

- `stripe.go`: client from `STRIPE_SECRET_KEY`; `CheckoutSession(userID, plan)`, `PortalSession(customerID)`.
- `stripe_webhook.go`: verify signature, switch on event type, upsert subscriptions/invoices, always 200 on handled (else Stripe retries).
- Use the official `github.com/stripe/stripe-go/v80` SDK (`go get` — I run `go mod tidy` after, sandbox limitation).
- Endpoints gated by the session middleware from Phase 2 (a checkout must belong to a logged-in user).

## Local testing (Phase 4)

- `stripe login` + `stripe listen --forward-to localhost:7700/api/stripe/webhook` → prints the webhook signing secret for `.env`.
- Test card `4242 4242 4242 4242`, any future expiry/CVC — completes a test subscription with zero real money.

## Sobhan's actions before REAL payments

1. In Stripe Dashboard → activate account (business details, bank = Revolut for payouts).
2. Confirm/adjust the 3 plan prices and currency (EUR now; PLN/USD possible).
3. Create the production webhook endpoint when we deploy, put its secret in prod `.env`.
