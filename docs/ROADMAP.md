# NOXIOAI — Master Roadmap & Build Guide

> **What is NOXIOAI:** AI employees for Iranian businesses. A bilingual (FA-first) platform where
> named AI characters — displayed as pixel-art people in a visual online office — handle real business
> tasks: marketing, social media (Instagram/Telegram), customer support, and development help.
> One shared "Brain" learns your business; every employee uses it. Run your company from one screen.
>
> **Domains:** noxioai.com (international) · noxioai.ir (Iran) — both on Cloudflare.
> **Owner:** Sobhan Azimzadeh (TECSO). **Built with:** Claude Code + the TECSO skill set.

---

## The Team (AI employees, v1 cast)

| Character | Persian | Role | Phase |
|---|---|---|---|
| **Nika** | نیکا | Marketing manager — campaigns, copy, SEO content | B |
| **Dara** | دارا | Developer — code help, site/app guidance, technical answers | B |
| **Sara** | سارا | Customer support — reply drafts, FAQ handling, tone-perfect Persian | B |
| **Avisa** | آویسا | Social media — Instagram/Telegram posts, calendars, captions | C |

More desks appear in later phases (sales, invoicing, recruiting...).

---

## Phases at a glance

| Phase | Name | Goal | Success metric (ONE number) | Status |
|---|---|---|---|---|
| **A** | Launch | Landing page + waitlist live on noxioai.com | 100 waitlist emails in 30 days | 🔨 in progress |
| **B** | The Office | Working product: auth + pixel office + 3 AI employees + Brain | Sobhan uses it daily for TECSO | ⬜ |
| **C** | Business | Automations + Instagram/Telegram tools + invite first users | 10 active outside users | ⬜ |
| **D** | Commercial | Zarinpal billing, plans, more employees, PWA | First paid Toman | ⬜ |

**Rule:** a phase starts only when the previous phase's metric is real (or consciously waived). Every phase ends with a FULL REPORT (template at the bottom).

---

## Phase A — Launch (target: 1–2 days)

**Deliverable:** animated bilingual landing page + working waitlist, deployed to Cloudflare Pages on noxioai.com.

### Technology
| Part | Tech | Why |
|---|---|---|
| Framework | **Nuxt 3** (SSG mode) | Sobhan's strongest stack; static output = free, fast, Iran-reachable |
| Languages | **@nuxtjs/i18n** — FA default (RTL) + EN | FA-first market; RTL from commit #1 is 5x cheaper than retrofitting |
| Styling | **Tailwind CSS** (logical properties only: `ms-*`, `pe-*`) | Fast + RTL-safe by construction |
| Motion | **@vueuse/motion** | Vue equivalent of framer-motion — spring animations, scroll reveals |
| Pixel art | CSS `box-shadow` pixel characters + keyframe animation | Zero image assets, crisp at any scale, tiny payload |
| Fonts | Vazirmatn (FA) + Inter (EN) | Persian glyph quality; letter-spacing 0 for FA |
| Waitlist | Web3Forms (free) → later D1/Postgres | Zero backend for v0; submissions land in Sobhan's email |
| Hosting | **Cloudflare Pages** | Domains already on CF; free; auto-deploy on every push to `main` |

### Day-by-day
- **Day 1 (build):** scaffold Nuxt + modules → landing sections (Hero / Meet the Team / Features / How it works / Waitlist) → FA + EN copy → RTL pass (rtl-i18n checklist) → push to `main`.
- **Day 2 (launch):** Sobhan connects Cloudflare Pages (checklist below) → custom domain noxioai.com → smoke test both locales on mobile → announcement posts (LinkedIn/Instagram/Telegram drafts provided) → Phase A report.

### Sobhan's checklist (things only you can do)
1. **Web3Forms key** — web3forms.com → enter email → copy key → give it to Claude (60s).
2. **Cloudflare Pages connect** — dash.cloudflare.com → Workers & Pages → Create → Pages →
   Connect to Git → pick `Tecso-Dev/Noxioai` → Build command: `npm run generate` →
   Output directory: `.output/public` → Save & Deploy.
3. **Custom domain** — in the new Pages project → Custom domains → add `noxioai.com` (CF auto-configures DNS since the domain is already there). Optionally add `noxioai.ir`.
4. **Publish the announcement posts** (drafts will be provided — publishing is human work).

### Definition of done
- [ ] Landing live at noxioai.com, both locales, mobile-perfect, RTL flawless
- [ ] Waitlist form stores/forwards emails and shows success state in FA/EN
- [ ] Lighthouse ≥ 90 performance/SEO · `hreflang` fa/en · OG images
- [ ] Phase A full report delivered

---

## Phase B — The Office (target: 2–4 weeks)

**Deliverable:** the real product, used daily by user #1 (Sobhan): login → your pixel office → chat with Nika, Dara, Sara → they answer using your business Brain.

### Technology
| Part | Tech | Why |
|---|---|---|
| App | Nuxt 3 **full-stack** (Nitro server routes) | One codebase, API + UI together |
| Database | **PostgreSQL** + Drizzle ORM | Real product data: users, chats, Brain profiles (postgres-patterns skill) |
| Auth | **nuxt-auth-utils** (sealed sessions) | Simple, secure, no JWT complexity for v1 |
| AI layer | **Pluggable provider interface** — open-weight models (Qwen / DeepSeek / Llama family) | Strong Persian + no US-sanctions fragility; provider swappable by env config |
| Brain | Structured business profile (JSON schema) injected into every employee's system prompt | Sintra's best idea, Persian-native |
| Office UI | Canvas/CSS isometric pixel office; each employee = desk; click desk → chat panel | The product's visual identity |
| Hosting | Cloudflare Pages (UI) + VPS or CF Workers for API/DB | Decided by latency test from Iran in week 1 of B |

### Build order (each step = a PR Sobhan reads + a part-report)
1. App skeleton: auth, DB schema (users, brain_profiles, conversations, messages), layout shell
2. The Brain: onboarding form (business type, tone, products, audience) → stored profile
3. Employee engine: one shared chat pipeline (system prompt = character + Brain + task tools), then Nika/Dara/Sara as configurations
4. Pixel office UI: office scene, desks, click-to-chat, working/idle animations
5. Security pass (`security-review`): rate limits on AI endpoints, input validation, secrets audit
6. Sobhan-as-user week: daily real use for TECSO tasks → fix list → Phase B report

**Phase B gate:** detailed implementation plan via `claude-mem:make-plan` BEFORE coding, approved by Sobhan (Q method).

---

## Phase C — Business features (target: +3-4 weeks)

Avisa (social employee) · content calendar for Instagram/Telegram · automations (customer reply flows, seller follow-ups) · waitlist → invite system · onboarding polish · Persian SEO content engine begins (seo-geo-marketing workflows).
**Gate metric from B:** Sobhan actually uses it daily.

## Phase D — Commercial (target: +4 weeks)

Zarinpal billing (Toman) · plan tiers (desk count = plan) · usage limits · PWA install · more employees · GEO setup (llms.txt, entity schema) so AI engines know what NOXIOAI is.
**Gate metric from C:** 10 real users.

---

## Project structure (repo layout as of Phase A)

```
Noxioai/
├── README.md              ← public coming-soon / project face
├── docs/
│   └── ROADMAP.md         ← THIS FILE — the master guide
├── nuxt.config.ts         ← framework config (i18n, tailwind, motion)
├── package.json
├── i18n/locales/
│   ├── fa.json            ← Persian copy (primary)
│   └── en.json            ← English copy
├── pages/index.vue        ← the landing page
├── components/landing/    ← Hero, Team, Features, HowItWorks, Waitlist
├── assets/css/            ← Tailwind + pixel-art helpers
└── public/                ← favicon, OG images
   (Phase B adds: server/, db/, composables/, office/)
```

## Working rules (how we develop — agreed 2026-07-05)

1. **Q method** gates every phase and every non-trivial feature: analyze → questions → spec → Sobhan approves → build → verify.
2. **Full reports:** a part-report after every build step, a FULL phase-report at every phase end (template below). No silent progress.
3. **Git:** feature branches → small PRs with plain-language descriptions (Sobhan reads them to learn) → `main` always deployable → `/code-review ultra` (run by Sobhan) after each phase.
4. **One metric per phase.** Never more.
5. **Honesty rules:** no invented numbers anywhere (site copy included); unfinished = reported as unfinished.

## Phase report template

```
PHASE X REPORT — <date>
✅ Delivered: (vs the spec, line by line)
⚠️ Not done / moved: (what + why + where it went)
🔢 Metric: (current number vs target)
🧠 Learned: (what changed our thinking)
▶️ Next: (first 3 steps of next phase)
🙋 Needs Sobhan: (actions only you can do)
```
