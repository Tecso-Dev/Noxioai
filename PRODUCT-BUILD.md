# NOXIOAI Product Build — Self-Serve "AI Sales Employee"

Decided 2026-07-16 (owner): build the real multi-tenant self-serve product. Build in demoable slices so each milestone is also a sales asset.

## The core value (v1)
A business signs up → tells us their business → gets their own AI sales employee that finds leads, scores them, and drafts personalized outreach for THEIR business — with a human approval gate. "Customer #1 is us" (JARVIS) becomes a product anyone can use.

## THE critical architectural fact
Today the CRM tables (`companies`, `contacts`, `leads`, `outreach`, `experiences`) have **no owner** — every row is implicitly Sobhan's. The platform is single-tenant. Two blockers to multi-tenancy already in the schema:
1. **No `owner_id`** on any CRM table → customers would share one global pipeline.
2. **Global UNIQUE constraints** — `companies.website UNIQUE`, `leads.company_id UNIQUE` — two customers targeting the same company would collide.

## THE security boundary (non-negotiable)
Tenant isolation is the #1 requirement. Customer A must NEVER see customer B's leads. Every CRM read/write filters by the authenticated session's `owner_id`. A bug here is a data breach. This is designed in from line 1, not bolted on.

## ⚑ STRATEGY PIVOT 2026-07-18 — Iran-first wedge (see ~/Documents/noxioai-GTM.md)
First market = **Iran + region** (Turkey, Iraq/Afghanistan, Gulf — all 4 product locales already exist). Iranian SMBs live on **Telegram & Instagram**, not email/LinkedIn. So the WEDGE agent we sell first is the **Customer-Response agent** (Telegram-first: answers customers 24/7, captures leads, drafts content) — NOT the Sales/outreach agent (that stays the tool Sobhan uses to get his OWN customers). Seed already built = the Telegram support agent. Model = agent marketplace: launch with this ONE agent working, rest "coming soon". Pricing = simple flat tiers, **Toman via Zarinpal** for Iran + USD/crypto international. Billing bottleneck = customers/distribution, not tech.

## Milestone 1 — "Your AI Sales Employee" (SUPERSEDED as the wedge — see pivot above; multi-tenant foundation P1 still valid & done)

### Phase P1 — Multi-tenant foundation (invisible, load-bearing)
**Status 2026-07-16:** Local implementation and verification complete on the
41-lead production-copy database. Production backup, migration, binary rollout,
and smoke test remain approval-gated; P2 has not started.

- Add `owner_id BIGINT REFERENCES users(id)` to companies, contacts, leads, outreach, experiences (idempotent migration).
- Replace global uniques with per-owner composite uniques: `(owner_id, website)`, `(owner_id, company_id)`.
- Create Sobhan's owner user; backfill all existing 41 leads / 56 contacts to him (his data stays his).
- `ownerFromSession(r)` helper: every CRM query takes an owner_id; add a lint/test that fails if a CRM query lacks an owner filter.
- **DoD:** two test users each see only their own (empty) pipeline; Sobhan's data intact under his account; isolation test passes.

### Phase P2 — Confirm-gate + guided onboarding + real pipeline view
- **Hard email-confirm gate (owner asked 2026-07-18):** signup creates the account but grants NO app access until the emailed confirm code/link is used. `/app` and onboarding redirect unverified users to a "check your email" state; verify sets users.verified_at and unlocks.
- **Guided business-profile onboarding** (new `/onboarding` page, shown once after first verified login, then editable in account): multi-step form collecting everything we need about their business — what they sell, ideal customer, target city/country, primary language, website, current channels, goals, monthly lead target. EACH question has a short helper line explaining WHY we ask (sets expectation + improves answer quality). Store in a new `business_profiles` table (owner_id FK, one row per user). Data saved for future agent use AND flagged for founder review to improve the product (this is the feedback loop the owner wants).
- Dashboard `/app` becomes a real, tenant-isolated pipeline: leads table (empty at first), lead detail, status column. Locale-aware, RTL. Uses the profile to personalize.
- **DoD:** unverified user cannot reach /app; verified new user completes the guided profile, it persists, they see their own empty pipeline; no cross-tenant leakage; RTL + all 4 locales.

### Decisions 2026-07-18
- **JARVIS cockpit HUD:** expose behind ADMIN login only (users.is_admin + session), restyled to the premium-tech theme (currently sci-fi cyan). Not public. Scoped/served carefully re: which data it shows under the multi-tenant model. Do AFTER P2 (auth/tenant model matures first).
- **Light theme:** DEFERRED by owner — dark luxury is the brand identity; revisit after paying customers. Do not build now.

### Phase P3 — Agent actions in the dashboard
- "Find leads" button → ORACLE runs scoped to the user's profile → their leads appear in their pipeline.
- Lead detail → "Draft outreach" → ATLAS drafts (with the website fact-check step) → approval gate in the UI.
- Agent endpoints (/api/oracle, /api/atlas, /api/approve) become session-authenticated + owner-scoped (updates the edge lockdown: these open ONLY to authenticated sessions, still owner-filtered). Per-user LLM budget guard.
- Approved drafts: copy-to-send for v1 (defer per-tenant email deliverability to M2).
- **DoD:** a customer signs up → profile → find leads → draft → approve, entirely in the browser, seeing only their data.

## Milestone W (the Iran wedge — the sellable product) — supersedes M1 as launch target
Build order on top of P1 (done):
- **PW-a (NEXT BUILD): per-tenant business profile + knowledge base.** Confirm-gated /onboarding page: guided form (business name, what they sell, ideal customer, city/country, language, website, Telegram handle, and a KNOWLEDGE box = the FAQ/answers the customer-agent will reply from), each field with a "why we ask" helper. Stored in new `business_profiles` table (owner_id FK). Persian-first, RTL, 4 locales. Editable in /account.
- **PW-b: per-tenant Customer-Response agent.** Each customer connects their own Telegram bot (their BotFather token, stored per-tenant) → the agent answers their customers from their knowledge base (DeepSeek), captures leads, escalates to them. Anything it SENDS is gated. This is the productized, multi-tenant version of the existing support agent.
- **PW-c: Persian AI concierge** in the dashboard — helps them set up + connect (a scoped support-bot on the site).
- **PW-d: Zarinpal billing** (Toman) for Iran + gate the agent behind active subscription; USD/crypto for international.
- Marketplace UI: show the catalog (Customer-Response = live; Sales/SEO/Social = "coming soon").

## Milestone 2 (later — after wedge has paying customers)
Sales/SEO/Social agents as self-serve, usage metering, expansion tooling. Not built until the wedge has real paying customers telling us what they need.

## What NOXIOAI needs from Sobhan
- **Now:** approval of this plan + P1 start.
- **P3:** confirm per-user LLM budget cap (protect the DeepSeek balance from a runaway/abusive user).
- **M2:** Stripe live keys; decision on per-tenant sending (shared vs. their own).

## Build discipline
One phase at a time, each gated by its DoD + Sobhan's review. Foreman-orchestrated (LEAD plans/verifies, workers execute). Tenant-isolation test is mandatory on every phase touching CRM data. No secret in git. RTL tested every phase.
