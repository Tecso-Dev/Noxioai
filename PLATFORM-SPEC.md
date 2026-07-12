# NOXIOAI Platform — Specification (source of truth)

**Version 1.0 — 2026-07-12 — Status: approved direction, Phase 1 not started**

Every implementation session starts by reading this file. If code and spec
disagree, fix one of them in the same session — never let them drift. This is
the platform-level companion to `jarvis/SPEC.md` (the engine spec).

Decisions locked by Sobhan 2026-07-12: **Nuxt/Vue stack · Stripe payments ·
full spec before code.**

---

## 1. What we are building

Turn noxioai.com from a static waitlist page into a **real SaaS platform**:
a cinematic public site, customer accounts, a subscription-billed customer
dashboard backed by the JARVIS engine, and an internal accounting/admin view.

**The connection that matters:** today the public Nuxt site and the Go JARVIS
engine live in one repo but don't talk. This platform makes the site an
authenticated front door to the engine — customers sign up, subscribe, pay,
and get a dashboard; JARVIS does the work behind it.

## 2. Stack (decided — do not re-litigate)

| Layer | Choice | Note |
|---|---|---|
| Frontend | **Nuxt 4 (Vue 3) + Tailwind** | already live; extend, don't rewrite |
| Motion | **@vueuse/motion + GSAP + Three.js** | Framer Motion is React-only → not used; these give the same cinematic result in Vue |
| i18n | **@nuxtjs/i18n** | already installed; RTL already wired for `fa` |
| Backend | **Go** (the JARVIS binary, extended) + net/http | one binary keeps ops simple |
| DB | **PostgreSQL** (the existing jarvis-db, new tables) | port 5434 |
| Payments | **Stripe** — hosted Checkout + Billing + webhooks | payouts reach the Revolut account by bank transfer |
| Auth | Go-issued **HTTP-only session cookies**, argon2id password hashes | no third-party auth SaaS in v1 |
| Deploy | Nuxt static/SSR + Go API; Hetzner CX32 when it goes 24/7 | localhost during build |

## 3. Design system — "spatial glass"

Carry the JARVIS HUD aesthetic into the marketing site so the brand is one
world, not two.

- **Glassmorphism:** frosted panels — `backdrop-filter: blur(14px)`,
  translucent fills (`rgba(7,24,39,.6)`), 1px hairline borders, corner
  brackets (reuse the HUD's `::before/::after` bracket motif).
- **Spatial UI:** depth via parallax layers, a 3D hero (Three.js — the
  data-sphere / agent-network entity already built for the HUD, reused as the
  landing centerpiece), floating cards that lift on hover, a subtle holo-grid
  floor.
- **Palette:** the project cyan system (`--cyan:#3ee1ff`, deep navy grounds)
  already in `assets/css/main.css` and the HUD — one palette everywhere.
- **Motion principles** (Emil Kowalski-school, enforced by PIXEL): restraint,
  purposeful micro-interactions, staggered reveals (30–50ms), spring easing,
  `prefers-reduced-motion` honored. No decorative-only animation.
- **Type:** Inter (Latin) + Vazirmatn (Persian/Arabic) already loaded; add a
  Turkish-safe fallback (Inter covers it).

## 4. Internationalization — EN · FA · TR · AR

Extend the existing nuxt-i18n config (`fa` default, `en` present):

- Add locales: `{ code:'tr', language:'tr-TR', dir:'ltr', file:'tr.json' }`,
  `{ code:'ar', language:'ar', dir:'rtl', file:'ar.json' }`.
- **RTL** for `fa` and `ar`: the site already flips `dir` per locale; audit
  every new component for logical properties (`margin-inline`, `text-align:
  start`) instead of left/right so RTL is automatic.
- Every user-facing string lives in `i18n/locales/*.json` — no hardcoded copy.
- Numbers/dates/currency via `Intl` with the active locale.
- **RTL is a first-class test**, not an afterthought — check FA and AR on
  every phase (this is Sobhan's stated edge; see `rtl-i18n` skill).

## 5. Architecture & data model

```
Browser ── Nuxt (SSR/static) ──HTTPS──▶ Go API (/api/*) ──▶ Postgres
                    │                          │
                    └── Stripe.js (Checkout) ──┘ (Stripe hosted pages only)
                                    ▲
                         Stripe webhooks ──▶ Go (/api/stripe/webhook)
```

New Postgres tables (added to `jarvis/schema.sql`, same DB):

```sql
CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  email         TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,            -- argon2id; NEVER plaintext
  name          TEXT,
  locale        TEXT DEFAULT 'en',
  stripe_customer_id TEXT,
  created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE sessions (
  token      TEXT PRIMARY KEY,            -- random 32B, sent as HTTP-only cookie
  user_id    BIGINT REFERENCES users(id),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE subscriptions (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT REFERENCES users(id),
  stripe_sub_id TEXT UNIQUE,
  plan          TEXT,                     -- starter | pro | agency
  status        TEXT,                     -- trialing | active | past_due | canceled
  current_period_end TIMESTAMPTZ,
  updated_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE invoices (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT REFERENCES users(id),
  stripe_invoice_id TEXT UNIQUE,
  amount_cents   INT,
  currency       TEXT,
  status         TEXT,                    -- paid | open | void
  hosted_url     TEXT,                    -- Stripe-hosted invoice PDF/page
  created_at     TIMESTAMPTZ DEFAULT now()
);
```

## 6. Payments — security boundary (non-negotiable)

1. **No card data ever touches our site, our server, or the assistant.**
   Card entry happens only on **Stripe's hosted Checkout / Billing Portal**.
2. We store only Stripe IDs and statuses — never PANs, never CVCs.
3. Subscription truth comes from **Stripe webhooks**, verified with the
   signing secret; the DB is a mirror, Stripe is the source.
4. The Go webhook handler verifies `Stripe-Signature` before trusting any event.
5. Secrets (`STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`) live in `.env`
   (gitignored), never in the repo, never in client code. Only the
   **publishable** key reaches the browser.
6. **Sobhan's action, not mine:** create the Stripe account, define products/
   prices, obtain keys, connect the Revolut bank account for payouts. I cannot
   create the merchant account or move money.

## 7. Phases

### Phase 1 — Landing redesign + i18n
Rebuild the public site in the spatial-glass system: 3D hero (reuse the
JARVIS entity), glassmorphism sections, GSAP/@vueuse staggered motion,
responsive, all four languages with RTL for FA/AR.
**Done when:** noxioai.com renders the new design at all breakpoints, every
string switches across EN/FA/TR/AR, FA+AR read correctly RTL, Lighthouse
perf ≥ 85, reduced-motion honored.

### Phase 2 — Auth & accounts
Go API: `POST /api/auth/signup|login|logout`, argon2id hashing, session
cookies. Nuxt pages: signup, login, account settings (name, locale, password).
**Done when:** a user can register, log in, stay logged in across reloads, edit
their account, and log out; passwords are argon2id; sessions expire.

### Phase 3 — Customer dashboard
Authenticated `/app` area: the customer's view of JARVIS (their briefings,
their status), account overview, locale-aware. Gated by session middleware.
**Done when:** a logged-in user sees a personalized dashboard; anonymous users
are redirected to login.

### Phase 4 — Billing (Stripe)
Pricing page (3 plans), Stripe Checkout for subscribe, Billing Portal for
manage/cancel, invoices list from the `invoices` table, webhook handler
keeping `subscriptions`/`invoices` in sync.
**Done when:** a test-mode customer subscribes via Stripe Checkout, the
dashboard reflects active status from a webhook, an invoice appears, and
cancel via the Billing Portal flips status — all with test cards, zero card
data in our DB.

### Phase 5 — Admin & accounting
Internal-only `/admin` (Sobhan): customers, MRR, active subscriptions,
invoice/payment status, revenue over time. Read-mostly, session-gated to an
admin flag.
**Done when:** Sobhan sees live revenue, subscriber count, and payment status
pulled from the billing tables.

**Everything past Phase 5** (team accounts, usage metering, more providers,
mobile app) is expansion justified only by real customers.

## 8. Build discipline

- One phase at a time, each gated by its Definition of Done and Sobhan's review.
- Delegate mechanical/multi-file work to **Codex**; Claude does architecture,
  visual/UX, verification, and every commit. Verify before commit; **push to
  GitHub every verified round.**
- Reuse before building: the HUD's Three.js entity, glass panels, color tokens,
  and i18n scaffold already exist — extend them.
- No secret in git. No card data anywhere but Stripe. RTL tested every phase.

## 9. What NOXIOAI needs from Sobhan (blocking, per phase)

- **Phase 4:** a Stripe account, 3 products/prices defined, publishable +
  secret keys, webhook signing secret, Revolut bank connected for payouts.
- **Translations:** FA is native; EN present; **TR + AR need review by a
  fluent speaker** before launch (machine translation drafts are a start, not
  shippable copy).
- **Go-live:** the Hetzner VPS (CX32) when the platform should run 24/7.
