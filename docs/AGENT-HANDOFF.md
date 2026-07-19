# Agent coordination handoff

This file is the shared coordination point for all AI coding sessions working
in this repository. Read it before changing code, update it when taking
ownership of work, and record a concise handoff when stopping.

## Current state

- **Branch:** `main`
- **Latest remote commit:** `6d8e36a` — Vercel `/api` proxy uses
  `api.noxioai.com`.
- **Internal engine:** JARVIS v1 is live; the current product direction turns
  its sales workflow into the multi-tenant self-serve product specified in
  [PRODUCT-BUILD.md](../PRODUCT-BUILD.md).
- **Public product:** Nuxt frontend, session auth, transactional email,
  dashboard shell, and Stripe test-mode billing are live. The public edge
  exposes only approved auth/billing routes; agent-control routes remain
  blocked pending the authenticated P3 UI.
- **Worktree notice:** P1 is implemented and locally verified but uncommitted
  on `main`. The dirty `jarvis/` CRM/agent files, `schema.sql`, the two tenancy
  tests, `PRODUCT-BUILD.md`, and this coordination documentation belong to the
  P1 change set. Do not mix unrelated edits into them.
- **Reference documents:**
  - [Product roadmap](ROADMAP.md)
  - [Approved public tech stack](TECH-STACK.md)
  - [JARVIS specification](../jarvis/SPEC.md)
  - [JARVIS operating guide](../jarvis/README.md)
  - [Platform specification](../PLATFORM-SPEC.md)
  - [Self-serve product build](../PRODUCT-BUILD.md)

## Coordination rules

1. Check `git status --short`, `git log --oneline -5`, and this file before
   editing.
2. Claim files or a self-contained area below before making a non-trivial
   change. Do not edit files actively owned by another session.
3. Use a dedicated branch per feature unless Sobhan explicitly asks for a
   direct `main` update.
4. Keep commits focused, with a clear conventional commit message. Never
   rewrite or discard another session's changes.
5. Before ending a session, update the handoff with the commit, verification,
   changed files, remaining work, and any decision Sobhan must make.
6. If ownership overlaps or the worktree contains unexpected changes, stop
   editing that area and resolve the handoff first.

## Active ownership

| Owner/session | Scope | Files or area | Branch | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| Codex | Product M1 / P1 multi-tenant foundation | Current dirty P1 files listed above | `main` | Local verification complete | Awaiting Sobhan's decision on commit and production rollout. P2 has not started. |
| Codex `/root` | Authentication security and UX refresh | `pages/login.vue`, `pages/signup.vue`, related auth UI/i18n, `jarvis/auth*.go`, auth tests, edge headers | `codex/auth-security-refresh` | Local implementation complete | Production rollout is gated on DB migration/backup and `JARVIS_AUTH_DATA_KEY`; OAuth/TOTP remain optional follow-up work. Preserve unrelated P1 and marketing work. |

## Handoff log

### 2026-07-19 — Codex `/root`

- **Completed:** Rebuilt login, sign-up, recovery, verification, and account
  security UI in EN/FA/TR/AR. Added email-or-username login, accessible password
  visibility controls, private-device remembrance, email verification, server-
  recorded legal consent, Argon2id parameter upgrades, breached/common password
  screening, enumeration-resistant responses, progressive throttling, hashed
  session/reset tokens, CSRF/origin checks, security headers, encrypted WebAuthn
  passkeys, active-session termination, audit events, and authentication-change
  notifications.
- **Files changed:** Auth pages/components/composable and global styles; all four
  locale files; `jarvis/auth.go`, `auth_email.go`, `auth_security.go`,
  `passkeys.go`, schema/tests and verified-only API guards; edge header configs;
  Nuxt TypeScript tooling. Existing landing showcase and marketing assets were
  preserved and are not part of this auth scope.
- **Commit/branch:** Uncommitted on `codex/auth-security-refresh`; baseline is
  `a9d2df0` with unrelated dirty work still present.
- **Verified:** `go test -race ./...`, `go vet ./...`, Nuxt type-check and
  production build, npm audit (0 reported vulnerabilities), JSON/key parity for
  503 locale keys, `git diff --check`, local API capability/header smoke test,
  and headless Chromium desktop/mobile/RTL form, keyboard, semantics, target-
  size, reduced-motion, and 320px reflow checks. PostgreSQL integration was not
  run because the local Docker daemon/database were unavailable. The optional
  exhaustive Codex Security workspace was opened but not started.
- **Remaining work:** Before production, back up PostgreSQL, run the idempotent
  schema migration, provision a random 32-byte `JARVIS_AUTH_DATA_KEY`, deploy
  backend plus Vercel/Caddy changes together, and smoke-test real email/passkey
  ceremonies. Configure trusted OIDC provider credentials and TOTP only if the
  product chooses those optional methods. Tighten CSP with per-request nonces in
  a future server-rendering pass; Nuxt currently needs `script-src 'unsafe-inline'`.
- **Decision needed from Sobhan:** Approve the separately gated production
  migration/deployment and provide the intended OAuth providers, if any.

### 2026-07-16 — Codex

- **Completed:** Resumed Claude's P1 work after its session-limit cutoff and
  completed the local multi-tenant foundation. Added owner scoping to all CRM
  call paths, session-only HTTP ownership, per-owner uniqueness, mandatory
  owner columns, database-enforced same-owner parent/child relationships, an
  atomic backfill/finalization transaction, and static/runtime regression
  tests. Web chat no longer exposes Sobhan's personal memory and now requires
  an authenticated session.
- **Files changed:** `PRODUCT-BUILD.md`, `.foreman/ledger.md`, this handoff,
  `jarvis/schema.sql`, `jarvis/db.go`, CRM agent/caller files, `jarvis/main.go`,
  `jarvis/hud.go`, `jarvis/db_test.go`, `jarvis/tenancy_test.go`, and
  `jarvis/tenant_security_test.go`. `jarvis/auth_email.go` has a one-character
  percent-escape fix needed for `go vet`.
- **Commit/branch:** Uncommitted on `main`; baseline and `origin/main` remain
  `6d8e36a`.
- **Verified:** `go vet`, build, full tests, race tests, explicit tenant and
  anonymous-route tests, static CRM-query guard, `git diff --check`, and two
  consecutive local migration runs. Production-copy counts remain 41
  companies, 56 contacts, and 41 leads; null-owner and relationship-mismatch
  counts are all zero.
- **Remaining work:** With separate approval, confirm the production owner
  email, build/upload a candidate binary, take a fresh database backup, stop
  the service, run the candidate's idempotent migration, activate it, restart,
  and smoke-test. Rollback is restore-the-backup plus the previous binary.
- **Decision needed from Sobhan:** Approve commit only, or approve commit plus
  the separately gated production rollout. Do not start P2 before this review.

### 2026-07-12 — Codex

- **Completed:** Enhanced the JARVIS HUD with interactive agent workspace,
  cinematic startup audio at 50% volume, and randomized original welcome
  messages. Updated product and JARVIS documentation; added redacted HUD
  screenshots.
- **Pushed:** `e3c3b8c` and `cb3e7f9` to `origin/main`.
- **Verified:** Go tests passed before documentation-only work; documentation
  links and staged diff checks passed; screenshots were visually reviewed.
- **Next decision for Sobhan:** The platform plan is now approved in
  `PLATFORM-SPEC.md`; Phase 1 is proceeding in a separate UI-only scope.

### 2026-07-12 — Codex

- **Claimed:** Phase 1 public landing redesign and localization polish.
- **Boundary:** Public Nuxt UI only. The active Go auth work remains reserved
  for the Claude session.
- **Source of truth:** `PLATFORM-SPEC.md`; the plan is approved and Phase 1
  already has its Turkish/Arabic i18n foundation in `4016668`.

### 2026-07-12 — Codex

- **Completed:** Spatial-glass landing redesign slice: cyan/navy system, grid
  and depth layers, a cinematic office-console hero, glass cards and FAQ
  states, and a refined waitlist surface. The small-screen navigation and
  stacked CTAs are responsive; new visible status labels are translated in
  EN/FA/TR/AR.
- **Files changed:** `app.vue`, `pages/index.vue`, `assets/css/main.css`,
  `components/landing/Landing{Hero,Features,Team,How,Faq,Waitlist,LangSwitcher}.vue`,
  and all four locale JSON files.
- **Verified:** `npm run build`, `npm run generate` (all locale routes),
  JSON translation-key parity, `git diff --check`, and local Chrome desktop
  rendering. The production document had no horizontal overflow in browser
  measurement.
- **Remaining work:** Reuse the JARVIS Three.js data-sphere as the live public
  hero entity and run Lighthouse on a deploy preview. No auth or JARVIS files
  were touched.

## Update template

Copy this section for every meaningful handoff.

```md
### YYYY-MM-DD — <owner/session>

- **Claimed/completed:**
- **Files changed:**
- **Commit/branch:**
- **Verified:**
- **Remaining work:**
- **Decision needed from Sobhan:**
```
