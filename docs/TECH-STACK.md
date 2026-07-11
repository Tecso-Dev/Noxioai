# NOXIOAI — Approved Technology Stack

> **Status: APPROVED by Sobhan, 2026-07-05.** This is the binding tech map for all phases.
> Changes require a conscious decision recorded here. Rationale considers: team skills
> (Vue expert, Go learner), Iran constraints (sanctions, payments, connectivity), solo-founder budget.

## Languages
**TypeScript** (frontend) · **Go** (backend) · **SQL** (schema/queries) · **Bash + YAML** (ops/CI).
Deliberately excluded: Python, PHP. Frontend framework decision: **Vue (stay)** — React ruled out; Vue equivalents (Inspira UI, shadcn-vue, @vueuse/motion) cover every desired React-ecosystem capability.

## Current internal JARVIS implementation

JARVIS in [`jarvis/`](../jarvis/README.md) is a deliberately lean internal
validation engine, not the final public Phase B architecture. It uses Go's
standard `net/http`, `database/sql` with pgx, PostgreSQL 16, an embedded
single-file HUD, and macOS launchd. This lets the sales workflow prove value
without prematurely adding public authentication, Redis, a queue, or
multi-user infrastructure.

The Phase B choices below remain the binding plan for the public Noxioai
Office. JARVIS is evidence for the design, not an unrecorded stack change.

## 1. Frontend
| Part | Technology | Notes |
|---|---|---|
| Framework | Nuxt 3 → 4, TypeScript | live since Phase A |
| Styling | Tailwind CSS (logical properties only) | RTL-safe |
| Components | shadcn-vue + Inspira UI | 21st.dev/Aceternity-style animated components, Vue-native |
| Motion | @vueuse/motion; GSAP for complex scroll scenes | |
| 3D (later) | TresJS (Three.js for Vue) | Phase C+ if the office goes full 3D |
| Pixel office | CSS sprites (now) → Canvas/PixiJS (Phase C) | walking/collaborating employees |
| State | Pinia | |
| i18n | @nuxtjs/i18n, FA default + RTL | rtl-i18n skill rules |

## 2. Backend (Go)
| Part | Technology | Notes |
|---|---|---|
| Language | Go 1.23+ | learning + shipping vehicle |
| Router | chi | stdlib-idiomatic |
| DB access | sqlc | type-safe generated Go from real SQL |
| Migrations | golang-migrate | plain SQL files |
| Auth | Session cookies, argon2id, alexedwards/scs | no JWT for first-party app |
| Logging | slog (stdlib) | structured |
| API style | REST + SSE streaming for AI output | SSE survives Iranian ISPs better than WebSockets |

## 3. Data
| Part | Technology | Notes |
|---|---|---|
| Main DB | PostgreSQL 16 | Brain profiles as JSONB |
| Vector memory | pgvector (Phase C) | Brain RAG inside Postgres |
| Cache/queue | Redis + asynq (Phase C) | rate limits, background jobs |
| Backups | nightly pg_dump → Cloudflare R2 | |

## 4. AI engine (sanctions-proof by design)
| Part | Technology | Notes |
|---|---|---|
| Architecture | Provider-agnostic Go interface (OpenAI-compatible shape) | model swap = env var |
| Models | Qwen 2.5/3 (Persian quality) + DeepSeek (reasoning/cost) | open-weight family |
| Serving v1 | DeepSeek API + Qwen via DashScope | no GPU ops, cheap |
| Serving at scale | vLLM self-hosted (EU GPU) | independence when volume justifies |
| Embeddings | bge-m3 | strong Persian |
| Keys | server-side only | never in frontend |

## 5. Integrations (Phase C)
- **Telegram Bot API** — first integration (open, free, doubles as notifications)
- **Instagram** — v1 generates + schedules, user posts manually (Meta API from Iran = later fight; do not promise auto-posting)
- Email: Brevo/SMTP2GO free tier — verify Iran accessibility at need
- **Payments: Zarinpal (Toman)** Phase D; USDT for international later

## 6. Infrastructure
| Part | Technology | Notes |
|---|---|---|
| Frontend hosting | GitHub Pages (Actions pipeline) | live |
| API + DB | Hetzner VPS (EU) + Docker Compose: api · postgres · redis · Caddy | ~€8/mo, auto-TLS |
| CI/CD | GitHub Actions → image → SSH deploy | |
| Monitoring | Uptime Kuma → Grafana/Loki later | self-hosted |
| Kubernetes | **explicitly NOT until >1 server is genuinely needed** | |

## 7. Mobile (Phase D)
PWA via Nuxt (installable, no app store — fits Iran). If a store app is ever required: Capacitor wrapping the same Vue codebase. No Flutter / React Native.
