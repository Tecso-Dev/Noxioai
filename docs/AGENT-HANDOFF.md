# Agent coordination handoff

This file is the shared coordination point for all AI coding sessions working
in this repository. Read it before changing code, update it when taking
ownership of work, and record a concise handoff when stopping.

## Current state

- **Branch:** `main`
- **Latest remote commit:** `4016668` — Phase 1 i18n foundation: Turkish and
  Arabic draft locales, including RTL support.
- **Internal engine:** JARVIS v1 is shipped as an internal proving ground.
- **Public product:** the approved five-phase SaaS plan is in
  [PLATFORM-SPEC.md](../PLATFORM-SPEC.md). Phase 1 is the public landing
  redesign and localization pass; Phase 2 auth is being developed separately.
- **Worktree notice:** Uncommitted edits are currently present in
  `.claude/launch.json`, `jarvis/go.mod`, `jarvis/go.sum`, `jarvis/main.go`,
  `jarvis/auth.go`, and `jarvis/auth_test.go`. Treat them as reserved for the
  active auth work.
- **Reference documents:**
  - [Product roadmap](ROADMAP.md)
  - [Approved public tech stack](TECH-STACK.md)
  - [JARVIS specification](../jarvis/SPEC.md)
  - [JARVIS operating guide](../jarvis/README.md)
  - [Platform specification](../PLATFORM-SPEC.md)

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
| Codex | Phase 1 public landing redesign and localization polish | Public Nuxt UI: `pages/`, `components/`, `assets/`, and needed locale copy | `main` | In progress | No changes to JARVIS, auth, billing, or `.claude/`. |
| Claude session | Phase 2 auth foundation | `.claude/launch.json`, `jarvis/go.mod`, `jarvis/go.sum`, `jarvis/main.go`, `jarvis/auth.go`, `jarvis/auth_test.go`, `jarvis/docs/superpowers/` | — | In progress | Auth files are reserved until Claude records a handoff. |

## Handoff log

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
