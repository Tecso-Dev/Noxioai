-- JARVIS schema — SPEC.md §5 is the authority; keep them in sync.
-- Idempotent: safe to run on every `jarvis db init`.

CREATE TABLE IF NOT EXISTS companies (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  website    TEXT UNIQUE,
  industry   TEXT,
  country    TEXT,
  raw_notes  TEXT,                          -- LLM-extracted summary of the site
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS contacts (
  id         BIGSERIAL PRIMARY KEY,
  company_id BIGINT REFERENCES companies(id),
  name       TEXT,
  role       TEXT,
  email      TEXT,
  linkedin   TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS leads (
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

CREATE TABLE IF NOT EXISTS outreach (
  id         BIGSERIAL PRIMARY KEY,
  lead_id    BIGINT REFERENCES leads(id),
  channel    TEXT,                          -- email | linkedin
  draft      TEXT NOT NULL,
  approved   BOOLEAN DEFAULT FALSE,         -- human gate: nothing leaves unapproved
  sent_at    TIMESTAMPTZ,
  outcome    TEXT,                          -- no_reply | replied | meeting | won
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS experiences (
  id         BIGSERIAL PRIMARY KEY,
  agent      TEXT NOT NULL,                 -- oracle | atlas
  input      TEXT,
  decision   TEXT,
  result     TEXT,
  lesson     TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- ── NOXIOAI platform tables (PLATFORM-SPEC §5) ──────────────────────────────
CREATE TABLE IF NOT EXISTS users (
  id            BIGSERIAL PRIMARY KEY,
  email         TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,            -- argon2id; NEVER plaintext
  name          TEXT,
  locale        TEXT DEFAULT 'en',
  is_admin      BOOLEAN DEFAULT FALSE,
  stripe_customer_id TEXT,
  created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  user_id    BIGINT REFERENCES users(id),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS subscriptions (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT REFERENCES users(id),
  stripe_sub_id TEXT UNIQUE,
  plan          TEXT,
  status        TEXT,
  current_period_end TIMESTAMPTZ,
  updated_at    TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS invoices (
  id             BIGSERIAL PRIMARY KEY,
  user_id        BIGINT REFERENCES users(id),
  stripe_invoice_id TEXT UNIQUE,
  amount_cents   INT,
  currency       TEXT,
  status         TEXT,
  hosted_url     TEXT,
  created_at     TIMESTAMPTZ DEFAULT now()
);
