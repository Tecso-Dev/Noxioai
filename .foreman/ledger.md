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

## MISSION MADUSA: trend agent + content machine (2026-07-20)

Baseline: 791bf0dd → actual `791b0dd`, clean (untracked .vscode/, marketing png — untouched).
Mode: Codex-boosted (codex-cli 0.145.0-alpha.18; standing consent per user rule 2026-07-18, Codex subscription billing). Cross-family verify.
Spec: Q-method approved 2026-07-20. FA-first+EN · LEAD-seeded watch list · render-on-approval GPU (snapshot-based, orphan reconcile, hour cap).
Real check: `cd jarvis && go vet ./... && go build ./... && go test -count=1 ./...`

| ID | Task | Write set | Seat | Status |
|----|------|-----------|------|--------|
| M0 | Recon integration map | read-only | foreman-scout (a36852812b5527665) | DONE |
| M1 | Core: ALL madusa schema tables + madusa.go (YT/Reddit/HN ingest, momentum+stage math, brain MAP, approve/reject/creators helpers) + madusa_test.go | jarvis/schema.sql, jarvis/madusa.go, jarvis/madusa_test.go | Codex top | DISPATCHED |
| M2 | Wiring: main.go subcommands, brief.go MAP section | jarvis/main.go, jarvis/brief.go | sonnet | after M1+M3 |
| M3 | Machine: madusa_pack.go (FA+EN per-platform package + storyboard on approve) + madusa_render.go (Vultr snapshot lifecycle, ssh/scp shell-out, reconcile, deliver) + tests | jarvis/madusa_pack.go, jarvis/madusa_render.go, jarvis/madusa_pack_test.go | Codex top | after M1 |
| M4 | HUD: MADUSA agent tile | jarvis/hud.go, jarvis/web/hud.html | sonnet | DISPATCHED |
| M5 | Verify: vet+build+test gate, blind foreman-verifier vs original spec | — | verifier | after M2 |
| M6 | Deploy prod + GPU snapshot prep + first MAP | server | LEAD+netops | BLOCKED(YOUTUBE key, VULTR key) |

Attempts (append-only):
- 2026-07-20 M0 attempt 1 → DONE
- 2026-07-20 M1 attempt 1 → dispatched codex-rescue (background)
- 2026-07-20 M4 attempt 1 → dispatched foreman-worker sonnet (background)
- 2026-07-20 M1 attempt 1 → BLOCKED: Codex monthly usage limit (resets Jul 26 4:35pm). Standing rule: fall back to Claude workers. Re-route M1 → sonnet, attempt 2 after session reset. No files touched.
- 2026-07-20 M4 attempt 1 → LOST: Claude session limit (resets 3:40pm Europe/Warsaw). Baseline verified clean, no partial edits. Re-dispatch attempt 2 after reset.
- 2026-07-20 ~2:52pm: background sleep timer started to auto-resume dispatch at ~3:42pm. Seat plan unchanged otherwise; M1 quality bar: sonnet clears it for well-specified implementation (FRONTIER design already in ticket).
- 2026-07-20 6:16pm: user said continue; session budget fresh. M1 attempt 2 → foreman-worker sonnet (background). M4 attempt 2 → foreman-worker sonnet (background). M4 ticket amended with concurrent-build attribution note (M1 writes madusa files in same package).
- 2026-07-20 6:17pm M4 attempt 2 → DONE. 4-line diff (hud.go mkAgent + hud.html NET_HUE/NET_POS/AGENT_HUE), gofmt clean, go build exit 0. Pending LEAD diff review + M5 blind verify.
- 2026-07-20 6:2xpm M1 attempt 2 → DONE. 3 files (schema +103, madusa.go ~700, madusa_test.go ~190), 24/24 tests, vet+build+test green. LEAD spot-check: signatures + madusa_posts/renders schema match ticket exactly. 2 benign deviations noted (unused ownerID param; trends.velocity NULL).
- 2026-07-20 6:23pm M3 attempt 1 → dispatched foreman-worker sonnet (background): madusa_pack.go + madusa_render.go + tests. Money-path safety fully specified (defer-destroy, orphan reconcile, 15min boot cap, deadline ctx).
- 2026-07-20 6:3xpm M3 attempt 1 → DONE. 1191 lines across 3 new files, 8 tests/25 subtests PASS, gate green. Disclosed limit: live Vultr/ssh path compiles but unverified until deploy (by design). Deviation accepted: un-ageable orphans left alive (money-safety, near-unreachable branch).
- 2026-07-20 6:35pm M2 attempt 1 → dispatched foreman-worker sonnet (background): main.go madusa subcommands, brief section, 4 systemd units. Design: approve=flip only; render timer */15min + manual render cmd.
- 2026-07-20 6:4xpm M2 attempt 1 → DONE. main.go dispatch + brief section + 4 units, gate green. Deviations correct (README quiet on timers; no EnvironmentFile convention — binary loads /opt/jarvis/.env itself).
- 2026-07-20 ~6:50pm M5 attempt 1 → PASS_WITH_NOTES (verifier). All 10 criteria pass. 4 fixable findings batch-fixed INLINE by LEAD (below dispatch gate): madusaGet query-strip rewrap + global UA; madusaSSHKeyPath home-dir fallback; destroy-defer moved before renders INSERT; madusaTruncate rune-safe + test. Full gate incl. -race green after fixes. Targeted re-verify sent to same verifier.
- Keys: JARVIS_YOUTUBE_KEY + JARVIS_VULTR_KEY placed in prod /opt/jarvis/.env (600, 17 vars). Vultr token verified 200 from Mac; 401 from prod = ACL missing 95.179.242.172/32 (user's one remaining click). Known cosmetic ceilings accepted: trend_id unlinked in MAP re-send; ownerID param unused.
