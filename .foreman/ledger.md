# Foreman Ledger — Noxio Autopilot services page

- Date: 2026-07-15
- Mode: Full (Agent tool + real shell). Codex present but not consented this session → Claude seats only.
- Baseline commit: f76d2724e8666162d62326e3f90559375ec94624 (main)
- Pre-existing dirty state (MUST remain untouched): `M jarvis/brain.go`, untracked `.vercelignore`, `jarvis/brain_test.go`, `marketing/`
- Real check: `npm run build` (Nuxt)

## Tasks

| ID | Task | Seat | Status |
|----|------|------|--------|
| T1 | Recon scout (structure/i18n/nav/pricing/contact/design) | FAST haiku (a5b894c8135596e29) | DONE |
| T2 | Build /services page + i18n keys ×4 + nav link | WORKHORSE sonnet (a8c20bf9a8a9a265b) | DONE |
| T3 | Deterministic gate: npm run build | LEAD shell | PASS |
| T4 | Blind verify (foreman-verifier) | verifier (add6decd52b2cd416) | PASS (9/9) |
| T5 | Hotfix: `overflow-hidden` on services main wrapper (glow spill, 448px h-overflow in RTL) | LEAD inline (1-word, below dispatch gate) | DONE — applied AFTER T4; re-verified visually in browser (fa + en screenshots), not re-blind-verified |

## T2 write set

- `pages/services.vue` (new)
- `i18n/locales/en.json`, `i18n/locales/fa.json`, `i18n/locales/tr.json`, `i18n/locales/ar.json` (add keys)
- `components/landing/LandingHero.vue` (nav link only)

## Phase 2b tasks (2026-07-15 PM)

| ID | Task | Seat | Status |
|----|------|------|--------|
| T6 | Auth transactional emails: verify-on-signup + password reset (Go + Nuxt + i18n) | WORKHORSE sonnet | DISPATCHED (background) |

T6 write set: jarvis/auth_email.go (new), jarvis/auth.go, jarvis/schema.sql, pages/verify.vue (new), pages/reset.vue (new), pages/login.vue, i18n/locales/{en,fa,tr,ar}.json. MUST NOT touch jarvis/brain.go (user WIP), .env, no git commits.

## MISSION: full-site premium-tech redesign (2026-07-15 evening, user mandate: "redesign all, sync everything, professional UI/UX")

Design law: marketing/brand/DESIGN-SYSTEM.md (night/panel/ivory #f2efe8/gold #d4bf94/gold-deep #b39868/pulse cyan; jewelry-not-lightning; whitespace = luxury). Hero already done (commit pending sections).

| Phase | Scope | Write set | Status |
|---|---|---|---|
| R1 | Foundation: tokens in tailwind+css, unified type scale, buttons, cards, inputs — VALUES not class names | assets/css/main.css, tailwind.config.ts | DISPATCHED |
| R2a | Landing sections: Team/Features/How/Why/FAQ/Waitlist/Pricing/LangSwitcher | components/landing/* (not LandingHero/HeroScene) | after R1 |
| R2b | Pages: services, login, signup, verify, reset, account, app(dashboard) | pages/* | after R1, parallel w/ R2a |
| R2c | Email templates re-skin to match (gold/ivory) | jarvis/auth_email.go | after R1, parallel |
| R3 | Verify all (build+visual fa/en), EXPLICIT color-coherence sweep (user flagged mismatched palette mid-transition — every surface must use ONLY night/panel/ivory/gold/gold-deep/pulse-cyan/dim; grep for stray #8E2DE2/violet/old-blue hexes across components+pages+css), regenerate og.png+banner to match, single deploy | — | last |

Rule: R2a/R2b/R2c may NOT touch main.css (R1 owns it); section-scoped styles go in component <style> blocks if needed.

## Attempts (append-only)

- 2026-07-15 T1 attempt 1 → DONE (report synthesized into T2 ticket)
- 2026-07-15 T2 attempt 1 → dispatched sonnet, sync

## MISSION P1: multi-tenant foundation (2026-07-16, product build M1)
Baseline: 6d8e36a. Local test DB: docker jarvis-db :5434 (copy of prod, 41 leads). Production server DB: DO NOT TOUCH during build — migration runs only after verification, with a backup first.
Security boundary: tenant isolation. 38 CRM query sites across brief/db/inbox/herald/hud/followup/caleb.go — EVERY one must filter by owner_id. Isolation test is the safety net + mandatory.
| ID | Task | Seat | Status |
|----|------|------|--------|
| P1 | schema owner_id + per-owner uniques + backfill + owner-scope all CRM queries + isolation test (LOCAL db only) | Sonnet worker + Codex hardening | DONE (local) |
| P1-verify | vet + build + full/race tests + isolation/security tests + migration idempotency + DB audit | Codex | PASS |
| P1-prod | backup prod, run migration with candidate binary, activate binary, smoke test | LEAD (separate approval) | AWAITING OWNER APPROVAL |

P1 verification evidence: `go vet ./...`, `go build ./...`, `go test -count=1 ./...`,
and `go test -race -count=1 ./...` pass. The local production-copy DB retains
41 companies / 56 contacts / 41 leads; all five CRM tables have zero null
owners, all owner columns are `NOT NULL`, and cross-owner relationship mismatch
counts are zero. Migration ran twice successfully. No production access,
commit, push, or deployment was performed.
