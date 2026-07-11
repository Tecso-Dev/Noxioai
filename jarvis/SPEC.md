# JARVIS — Specification (source of truth)

**Version 1.1 — 2026-07-12 — Status: v1 complete ✅ · internal daily-use engine · public Noxioai Office still planned**

Every implementation session starts by reading this file. If code and spec
disagree, fix one of them in the same session — never let them drift.

For setup, commands, the HUD surface, and redacted screenshots, see
[README.md](README.md). This file remains the product contract.

---

## 1. What JARVIS is

JARVIS is a lean agent-based business system whose only job in v1 is to fix a
**sales problem**: find qualified leads, draft outreach personalized enough to
get replies, and report progress daily. It is the **internal instance of the
Noxioai Phase B engine** — the same Go + Postgres core, pointed first at
Sobhan's own pipeline. When Phase B productizes, this engine is the seed, not
a rewrite.

It extends the existing JARVIS v0.2 (`main.go`, `brain.go`, `memory.go` —
435 lines, working REPL + HTTP + learning memory). Nothing is rewritten;
everything below is added around it.

## 2. What v1 is NOT (cut list)

| Cut | Add back when |
|---|---|
| Qdrant / vector DB | keyword search over `experiences` measurably fails; then pgvector, not Qdrant |
| Redis | a real cache need appears; until then in-process maps |
| n8n | never — Go cron/launchd covers scheduling |
| Next.js dashboard | SHIPPED 2026-07-11 by user decision as a zero-dependency HUD instead (`web/hud.html`, one file, no framework) — Next.js still cut |
| Multi-provider AI router | already exists: `JARVIS_BASE_URL`/`JARVIS_MODEL` env vars. It's configuration, not code |
| PIXEL / STARK as separate heavy agents | keep responsibilities as small persona prompts until repeated use proves dedicated infrastructure is needed |
| Agent registry / workflow engine / task queue | 5+ agents exist; until then a Go interface and a switch |
| Microservices, Kubernetes, multi-user SaaS | paying customers exist for the engine itself |
| Excessive documentation | never — this file is the product contract; a short [README](README.md) documents setup and operations |

## 3. Architecture

- **One Go binary** in `jarvis/` of the Noxioai monorepo
  (`~/Documents/noxioai/jarvis` — merged 2026-07-11; Nuxt site at repo root,
  engine here: one project). Subcommands:
  `jarvis` (REPL), `jarvis serve`, `jarvis oracle "<niche>"`,
  `jarvis atlas <lead-id>`, `jarvis approve <outreach-id>`, `jarvis brief`.
- **Postgres** via docker-compose, host port **5434** (5433/6380 are taken by
  DIGIKALA). Schema in `schema.sql`, applied by `jarvis db init`.
- **Brain**: the existing OpenAI-compatible client. Local Ollama for chat;
  point `JARVIS_BASE_URL`/`JARVIS_MODEL` at DashScope/DeepSeek/OpenRouter when
  ORACLE extraction needs a stronger model. Config, not code.
- **UI**: Telegram bot (briefings + approvals) plus the existing REPL/HTTP.
  Env: `JARVIS_TELEGRAM_TOKEN`, `JARVIS_TELEGRAM_CHAT`.
- **Scraping**: plain HTTP fetch first; Playwright (already installed) only
  for JS-heavy sites. No ScrapeGraphAI, no Python sidecar.
- **HUD** (`jarvis serve`, http://127.0.0.1:7700): local command center —
  `web/hud.html`, Three.js, and the startup audio are embedded in the binary.
  It has browser-native voice in/out (no Whisper/TTS stack), reactive data
  sphere and agent network, lead board, approval gate, live activity toasts,
  clickable agent dossiers, and a rotating original welcome sequence.
  Browser autoplay may require the explicit **Enable Startup SFX** control.
  It is local-only by default.
- **Runtime home `~/Library/JARVIS/`** (binary, .env, memory): macOS TCC
  blocks launchd agents from ~/Documents, so LaunchAgents
  (`com.noxioai.jarvis.serve` KeepAlive + `com.noxioai.jarvis.brief` 08:00)
  run the copy deployed by `./deploy.sh`. Repo stays the source of truth.

## 4. Agent contract

```go
type Agent interface {
    Name() string
    Run(ctx context.Context, task Task) (Result, error)
}
```

Dispatch is a `switch` in main. Two agents ship in v1: ORACLE and ATLAS.
FRIDAY is not an agent; it is one function on a timer.

## 5. Database schema

```sql
CREATE TABLE companies (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  website    TEXT UNIQUE,
  industry   TEXT,
  country    TEXT,
  raw_notes  TEXT,                          -- LLM-extracted summary of the site
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE contacts (
  id         BIGSERIAL PRIMARY KEY,
  company_id BIGINT REFERENCES companies(id),
  name       TEXT,
  role       TEXT,
  email      TEXT,
  linkedin   TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE leads (
  id               BIGSERIAL PRIMARY KEY,
  company_id       BIGINT UNIQUE REFERENCES companies(id),
  score            INT CHECK (score BETWEEN 0 AND 100),
  tier             TEXT,                    -- derived: 0-39 LOW, 40-69 MEDIUM, 70-89 HIGH, 90+ VIP
  reasoning        TEXT NOT NULL,           -- ORACLE must explain every score
  observed_problem TEXT,                    -- the hook ATLAS personalizes around
  suggested_offer  TEXT,
  status           TEXT DEFAULT 'new',      -- new → contacted → replied → won | lost
  created_at       TIMESTAMPTZ DEFAULT now(),
  updated_at       TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE outreach (
  id         BIGSERIAL PRIMARY KEY,
  lead_id    BIGINT REFERENCES leads(id),
  channel    TEXT,                          -- email | linkedin
  draft      TEXT NOT NULL,
  approved   BOOLEAN DEFAULT FALSE,         -- human gate: nothing leaves unapproved
  sent_at    TIMESTAMPTZ,
  outcome    TEXT,                          -- no_reply | replied | meeting | won
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE experiences (
  id         BIGSERIAL PRIMARY KEY,
  agent      TEXT NOT NULL,                 -- oracle | atlas
  input      TEXT,
  decision   TEXT,
  result     TEXT,
  lesson     TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);
```

## 6. Principles (non-negotiable)

1. **Human approval before anything leaves the machine.** The `approved`
   flag is the gate; there is no auto-send path in v1.
2. **Every score and draft explains itself.** `reasoning` and
   `observed_problem` are NOT NULL in spirit: an ORACLE run that can't say
   *why* a company is a good lead discards the lead.
3. **No generic outreach.** A draft must cite the company by name, the
   observed problem, the suggested solution, and the business value — or
   ATLAS regenerates it.
4. **Learning is experience retrieval, not retraining.** Every agent run
   appends an `experiences` row. Before scoring or drafting, the agent
   retrieves similar past experiences (SQL `ILIKE`/full-text first) and
   injects them into its prompt.
5. **Local-first.** Secrets in env vars, `memory/` never committed, no
   external service holds the CRM.

## 7. Phases

### Phase 1 — Foundation
docker-compose (Postgres only, port 5434), `schema.sql`, `db.go`
(pgx or database/sql), `Agent` interface, `jarvis db init`.
**Done when:** `docker compose up -d && jarvis db init` succeeds and
`go test ./...` passes with one round-trip DB test.

### Phase 2 — ORACLE (market intelligence)
`jarvis oracle "real estate agencies in Warsaw"` →
find candidate companies → fetch site → LLM-extract name/industry/contacts/
notes → score 0–100 with reasoning + observed problem + suggested offer →
upsert companies/contacts/leads → write experience row.
Scoring inputs: website quality, tech stack, online activity, apparent size,
fit with Noxioai/Tecso services.
**Done when:** one run puts ≥10 real scored leads in Postgres, each with a
reasoning a human agrees with.

### Phase 3 — ATLAS (outreach)
`jarvis atlas <lead-id>` → read lead + company + past experiences → draft
email and/or LinkedIn message honoring Principle 3 → store unapproved.
`jarvis approve <outreach-id>` flips the gate. Approved email can then be
dispatched by HERALD with `jarvis send <outreach-id>` when Gmail SMTP is
configured; LinkedIn remains manual. Outcome is recorded with
`jarvis outcome <outreach-id> replied`.
**Done when:** 5 approved drafts exist that cite real observed problems.

### Phase 4 — FRIDAY briefing + learning loop
`jarvis brief` → Telegram message: new leads by tier, drafts awaiting
approval, outcomes since yesterday, and CALEB's CRM-informed marketing memo.
Scheduled daily 8:00 via launchd. Experience retrieval is wired into ORACLE
scoring and ATLAS drafting prompts.
**Done when:** the briefing arrives on Telegram from the schedule, and an
ATLAS prompt visibly includes a retrieved past lesson.

**v1 is complete after Phase 4.** Everything else (dashboard, campaigns,
auto-send, semantic memory, multi-user) is expansion justified only by usage.

### Implementation snapshot — 2026-07-12

| Capability | State |
|---|---|
| Foundation, ORACLE, ATLAS, FRIDAY | v1 complete |
| Human approval gate | Enforced in the database and HERALD send path |
| CALEB | Shipped: daily CRM memo and `jarvis caleb` |
| HERALD email | Shipped: approved-email Gmail SMTP dispatch only |
| Command center | Shipped: local HUD, agent workspace, dossiers, activity, voice, startup sequence |
| PIXEL / HERALD social | Not built; intentionally next only after current value is proven |

## 8. v2 — persona agents & daily operations (build AFTER Phase 4)

Rule: a persona agent is a system-prompt file in `personas/` plus the same
`Agent` interface — one prompt file and one switch case, never new
infrastructure. Personas emulate a public figure's published thinking style;
they are inspiration, not the person.

**Morning boot (extends Phase 4):** launchd starts `jarvis morning` at 08:00
→ CALEB analysis + FRIDAY brief + queued drafts land in ONE Telegram message
with approve commands. Iron-Man tone lives in the persona prompts.

**CALEB — marketing strategist** (Caleb Ralston-school brand thinking).
Reads CRM outcomes + site analytics + social stats; outputs a daily memo:
what's working, 3 concrete actions, today's content angle.
*Constraint:* without data feeds it produces generic advice — wire CRM
(Phases 1–3) and one analytics source before building it.

**PIXEL — design & motion critic** (Emil Kowalski-school taste: restraint,
micro-interactions, motion with purpose). Reviews UI/landing/posts before
publish. Pure persona prompt, zero infra — usable from Phase 2 via REPL.

**HERALD — publisher & inbox.** Honest platform matrix:

| Channel | Auto-publish | Auto-DM/reply | Reality |
|---|---|---|---|
| Email (Gmail API) | yes | draft→approve | fully feasible |
| YouTube (Data API) | yes (~6 uploads/day quota) | draft→approve comments | feasible |
| Instagram (Graph API) | yes — needs Creator/Business account | Messenger API, limited | feasible |
| LinkedIn | API approval is hard for individuals; assisted posting | **ToS bans DM bots → restriction/ban** | draft→approve, send manually/browser-assisted |
| Freelancer.com (official API) | bids possible | messages possible | auto-bid = spam trap; draft→approve protects the proposal-craft edge |

**Non-negotiable:** Principle 1 covers every outbound message. JARVIS drafts
and queues; one-tap Telegram approval sends. Auto-send without approval is
out of spec — unattended DMs are how LinkedIn/Freelancer accounts die, and
those accounts are the sales channel this whole system exists to feed.

Shipped after Phase 4: HERALD-email → CALEB → HUD workspace. Next order:
HERALD-social, then PIXEL formalization. One agent at a time, each proving
value before the next starts.

## 9. Implementation prompt template (per phase)

> Read SPEC.md fully. Implement **Phase N only** — nothing from later phases.
> Extend the existing code; do not rewrite brain.go or memory.go. Shortest
> working diff that meets the phase's Definition of Done. Each non-trivial
> component gets one runnable check (small `_test.go` or assert-based
> self-check). When done, run the Definition of Done commands and show output.
