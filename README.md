<div align="center">

  <img src="https://capsule-render.vercel.app/api?type=waving&color=0:0b0b12,50:b39868,100:d4bf94&height=220&section=header&text=NOXIOAI&fontSize=76&fontAlignY=35&animation=twinkling&fontColor=f2efe8" />

  **AI employees for Persian-speaking businesses — they answer your customers 24/7 while you sleep.**

  [![Live](https://img.shields.io/badge/Live-noxioai.com-d4bf94?style=for-the-badge)](https://noxioai.com)
  [![Market](https://img.shields.io/badge/First_market-Iran_%2B_region-48CAE4?style=for-the-badge)](#market--strategy)

</div>

---

NOXIOAI is a **multi-tenant SaaS** where a business signs up, tells its AI about itself, and connects its own Telegram bot — then the AI answers that business's customers 24/7 from its knowledge base, captures leads, and escalates to a human when needed. Persian-first, built for the channels Iranian and MENA businesses actually use (Telegram, Instagram), expanding across four languages (FA · EN · TR · AR).

The whole company runs on its own product: **JARVIS**, an autonomous Go sales engine, runs NOXIOAI's own outreach — customer #1 is us.

## What's live today

| Capability | Status | What it is |
|---|---|---|
| Landing + services | 🟢 Live | Premium-tech FA-first site, 4 locales, legal pages, SEO |
| Auth (multi-tenant) | 🟢 Live | Session auth, email verification, per-tenant data isolation (test-enforced) |
| **Auth security** | 🟢 Live | Breached-password screening (HaveIBeenPwned), brute-force rate-limiting, CSRF origin checks, security headers, audit log, passkeys/WebAuthn |
| Guided onboarding | 🟢 Live | Business profile + knowledge base per tenant, confirm-gated |
| **Customer-response agent** | 🟢 Live | Each tenant connects its own Telegram bot → AI answers its customers from its knowledge base via secure webhook (constant-time secret, tenant-isolated), with human-escalation |
| Setup concierge | 🟢 Live | In-dashboard Persian AI helper for connecting bots |
| Social content agent | 🟢 Live | Autonomous Persian posts + branded images → owner approval → auto-post to Telegram channel |
| Transactional email | 🟢 Live | Verify + reset from `hi@noxioai.com` via Resend (domain-authenticated) |
| JARVIS sales engine | 🟢 24/7 | Go/Postgres ops: lead research/scoring, drafted outreach (approval-gated), daily Telegram briefing, weekly SEO agent, encrypted nightly backups |
| Admin cockpit · onboarding wizard · billing paywall · analytics | 🔨 Building | See [PRODUCT-BUILD.md](PRODUCT-BUILD.md) |

_No fabricated metrics: this documents architecture and capabilities, not traction._

## Market & strategy

First market: **Iran + region** (Turkey, Iraq/Afghanistan, Gulf — all four product locales exist). The moat is that global incumbents (Zapier, Sintra, HubSpot) **can't serve this market** — sanctions + no Persian + no local payment. Strategy and go-to-market: sell the customer-response agent as the wedge, own the niche, add agents as customers pay. Full plan lives outside the repo (private).

## AI brains

- **Gemini (via OpenRouter)** — customer-facing agents (sharp Persian).
- **DeepSeek** — reserved for internal/simple tasks and future high-volume surfaces.
- **fal.ai** — branded post images for the social agent.
- Never Claude/GPT at runtime: the agents think with the models above, on the project's own keys. Claude Code + Codex build the agents; they don't *run* them.

## Architecture

```mermaid
flowchart LR
  U[Business owner] -->|signup + onboarding| K[(Business profile + knowledge base)]
  U -->|connects own Telegram bot| W[Webhook /api/tg/secret]
  C[Their customers] --> W
  W -->|tenant-isolated| BR[Gemini brain]
  BR --> A[Auto-answer + lead capture]
  A -->|needs human| ES[Escalate to owner]
  K --> BR
  subgraph Internal ops (NOXIOAI's own)
    J[JARVIS] --> OR[ORACLE research/score] --> AT[ATLAS drafts] --> G[Approval gate] --> HE[HERALD send]
  end
```

## Technology

| Area | Implementation |
|---|---|
| Frontend | Nuxt 3, Vue 3, TypeScript, Tailwind, `@nuxtjs/i18n` (FA/EN/TR/AR), `@vueuse/motion` — Vercel |
| Backend | Go, `database/sql` + pgx, PostgreSQL, OpenAI-compatible model interface, session auth |
| Multi-tenancy | `owner_id` on every CRM row, per-owner uniqueness, isolation enforced by test |
| Agents | Go services + systemd timers: customer-response (webhook), support concierge, JARVIS sales, social content, SEO, briefing, inbox, uptime |
| Email | Resend HTTPS API, branded template, `hi@noxioai.com` |
| Payments | Stripe (international) + Zarinpal (Iran/Toman) — location-based (in progress) |
| Ops | Hardened Ubuntu VPS (key-only SSH, ufw, fail2ban, auto-updates), Caddy/TLS with an edge allowlist, encrypted nightly Telegram backups, DR runbook in [deploy/](deploy/) |
| Deploy | Vercel (frontend) + VPS (API/agents/DB); `vercel.json` proxies `/api/*` to `api.noxioai.com` |

## Latest activity

<!-- ACTIVITY:START -->
_Auto-updated 2026-07-19 17:10 UTC_

- `6125460` docs: rewrite README for the actual product — multi-tenant AI-employee SaaS with the per-tenant Telegram customer-response agent as the live wedge (was the stale Phase-B Office framing) — 2026-07-19
- `135c69d` landing: hacker-themed error/404 page (glitch status code, terminal line, 4 locales, reduced-motion safe) + LandingShowcase polish — palette-matched to design tokens, seamless left→right code marquee, staggered card reveal, denser code — 2026-07-19
- `1feaf1f` JARVIS admin: online super-admin HUD gated by is_admin — requireAdmin on every agent-control endpoint (403 non-admin), GET /admin serves cockpit (302 /login if not admin), data scoped to ops owner; DB-free requireAdmin unit test; /admin rewrite to api.noxioai.com (Codex-spec'd, Claude-built + security-reviewed) — 2026-07-19
- `4652779` concierge: require verified user (auth-refresh follow-up) — 2026-07-19
- `610542c` auth-security refresh: HaveIBeenPwned breached-password screening, brute-force rate-limiting, CSRF origin checks, security headers (Caddy+app+Vercel), auth audit log, hardened input validation; matching frontend auth pages (Codex-built, Claude-verified + security-reviewed) — 2026-07-19
<!-- ACTIVITY:END -->

## Run locally

```bash
# frontend
npm install && npm run dev

# backend + JARVIS
cd jarvis
docker compose up -d
go build -o jarvis . && ./jarvis db init && ./jarvis serve   # http://127.0.0.1:7700
```

See [jarvis/README.md](jarvis/README.md) for every agent command and configuration.

## Safety & operating rules

- Tenant isolation is a security boundary: a customer can only ever touch its own data (enforced by test on every change).
- Nothing outbound (outreach, public posts) is sent before a human approves it.
- Secrets live in environment files, never committed. Webhooks verify a constant-time secret.
- Agents run on the project's own model keys — never on the tools that build them.

---

Built by [Sobhan Azimzadeh](https://github.com/sobhanaz) · dual-national (Iran/Poland), full-stack.
