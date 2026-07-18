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
ALTER TABLE users ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  user_id    BIGINT REFERENCES users(id),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS auth_tokens (
  token      TEXT PRIMARY KEY,
  user_id    BIGINT REFERENCES users(id),
  purpose    TEXT NOT NULL CHECK (purpose IN ('verify','reset')),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
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

-- Platform-global product waitlist; deliberately not tenant-owned CRM data.
CREATE TABLE IF NOT EXISTS waitlist (
  id         BIGSERIAL PRIMARY KEY,
  email      TEXT NOT NULL UNIQUE,
  name       TEXT,
  locale     TEXT DEFAULT 'en',
  source     TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- Platform-global NOXIOAI support conversations; deliberately not tenant-owned.
CREATE TABLE IF NOT EXISTS support_messages (
  id           BIGSERIAL PRIMARY KEY,
  chat_id      BIGINT NOT NULL,
  username     TEXT,
  customer_msg TEXT,
  bot_reply    TEXT,
  escalated    BOOLEAN DEFAULT FALSE,
  created_at   TIMESTAMPTZ DEFAULT now()
);

-- Platform-global reporting for NOXIOAI's own website; deliberately not
-- tenant-owned and never used as a publishing queue.
CREATE TABLE IF NOT EXISTS seo_reports (
  id           BIGSERIAL PRIMARY KEY,
  created_at   TIMESTAMPTZ DEFAULT now(),
  period       TEXT,
  clicks       INT,
  impressions  INT,
  avg_position NUMERIC,
  analysis     TEXT,
  blog_draft   TEXT
);

-- ── multi-tenant ownership (PRODUCT-BUILD.md Phase P1) ──────────────────────
-- Every CRM row belongs to exactly one platform user; per-owner uniques
-- replace the old global ones so two tenants can target the same company.
ALTER TABLE companies   ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id);
ALTER TABLE contacts    ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id);
ALTER TABLE leads       ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id);
ALTER TABLE outreach    ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id);
ALTER TABLE experiences ADD COLUMN IF NOT EXISTS owner_id BIGINT REFERENCES users(id);

ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_website_key;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'companies_owner_website_key' AND conrelid = 'companies'::regclass
  ) THEN
    ALTER TABLE companies ADD CONSTRAINT companies_owner_website_key UNIQUE (owner_id, website);
  END IF;
END $$;

ALTER TABLE leads DROP CONSTRAINT IF EXISTS leads_company_id_key;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'leads_owner_company_key' AND conrelid = 'leads'::regclass
  ) THEN
    ALTER TABLE leads ADD CONSTRAINT leads_owner_company_key UNIQUE (owner_id, company_id);
  END IF;
END $$;

-- Composite parent keys let PostgreSQL enforce that child rows reference a
-- parent belonging to the same owner, not merely an existing numeric id.
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'companies_owner_id_key' AND conrelid = 'companies'::regclass
  ) THEN
    ALTER TABLE companies ADD CONSTRAINT companies_owner_id_key UNIQUE (owner_id, id);
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'leads_owner_id_key' AND conrelid = 'leads'::regclass
  ) THEN
    ALTER TABLE leads ADD CONSTRAINT leads_owner_id_key UNIQUE (owner_id, id);
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'contacts_owner_company_fkey' AND conrelid = 'contacts'::regclass
  ) THEN
    ALTER TABLE contacts ADD CONSTRAINT contacts_owner_company_fkey
      FOREIGN KEY (owner_id, company_id) REFERENCES companies(owner_id, id);
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'leads_owner_company_fkey' AND conrelid = 'leads'::regclass
  ) THEN
    ALTER TABLE leads ADD CONSTRAINT leads_owner_company_fkey
      FOREIGN KEY (owner_id, company_id) REFERENCES companies(owner_id, id);
  END IF;
END $$;
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'outreach_owner_lead_fkey' AND conrelid = 'outreach'::regclass
  ) THEN
    ALTER TABLE outreach ADD CONSTRAINT outreach_owner_lead_fkey
      FOREIGN KEY (owner_id, lead_id) REFERENCES leads(owner_id, id);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_companies_owner   ON companies(owner_id);
CREATE INDEX IF NOT EXISTS idx_contacts_owner    ON contacts(owner_id);
CREATE INDEX IF NOT EXISTS idx_leads_owner       ON leads(owner_id);
CREATE INDEX IF NOT EXISTS idx_outreach_owner    ON outreach(owner_id);
CREATE INDEX IF NOT EXISTS idx_experiences_owner ON experiences(owner_id);
