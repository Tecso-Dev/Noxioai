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
