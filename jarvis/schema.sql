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
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema() AND table_name = 'users' AND column_name = 'verified_at'
  ) THEN
    ALTER TABLE users ADD COLUMN verified_at TIMESTAMPTZ;
    -- Accounts that existed before verification was introduced are grandfathered.
    UPDATE users SET verified_at = now();
  END IF;
END $$;
ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS webauthn_id BYTEA;
ALTER TABLE users ADD COLUMN IF NOT EXISTS terms_accepted_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS privacy_accepted_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS legal_version TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS users_username_lower_key ON users (lower(username)) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_webauthn_id_key ON users (webauthn_id) WHERE webauthn_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS users_google_sub_key ON users (google_sub) WHERE google_sub IS NOT NULL;

CREATE TABLE IF NOT EXISTS business_profiles (
  id             BIGSERIAL PRIMARY KEY,
  owner_id       BIGINT UNIQUE REFERENCES users(id),
  business_name  TEXT,
  sells          TEXT,
  ideal_customer TEXT,
  city           TEXT,
  country        TEXT,
  language       TEXT,
  website        TEXT,
  telegram       TEXT,
  knowledge      TEXT,
  goals          TEXT,
  created_at     TIMESTAMPTZ DEFAULT now(),
  updated_at     TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS tenant_bots (
  owner_id       BIGINT PRIMARY KEY REFERENCES users(id),
  bot_token      TEXT NOT NULL,
  bot_username   TEXT,
  webhook_secret TEXT NOT NULL UNIQUE,
  active         BOOLEAN DEFAULT TRUE,
  created_at     TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS tenant_messages (
  id            BIGSERIAL PRIMARY KEY,
  owner_id      BIGINT REFERENCES users(id),
  from_chat     TEXT,
  from_name     TEXT,
  customer_text TEXT,
  agent_reply   TEXT,
  escalated     BOOLEAN DEFAULT FALSE,
  created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tenant_messages_owner ON tenant_messages(owner_id);
CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  user_id    BIGINT REFERENCES users(id),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS session_id TEXT;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS user_agent TEXT;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ip_hint TEXT;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS remembered BOOLEAN DEFAULT FALSE;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS auth_method TEXT DEFAULT 'password';
CREATE UNIQUE INDEX IF NOT EXISTS sessions_session_id_key ON sessions (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions (user_id);
CREATE TABLE IF NOT EXISTS auth_tokens (
  token      TEXT PRIMARY KEY,
  user_id    BIGINT REFERENCES users(id),
  purpose    TEXT NOT NULL CHECK (purpose IN ('verify','reset')),
  expires_at TIMESTAMPTZ NOT NULL,
  used_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE TABLE IF NOT EXISTS passkeys (
  id              BIGSERIAL PRIMARY KEY,
  user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_id   BYTEA NOT NULL UNIQUE,
  credential_data BYTEA NOT NULL,
  name             TEXT NOT NULL,
  created_at       TIMESTAMPTZ DEFAULT now(),
  last_used_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS passkeys_user_id_idx ON passkeys (user_id);
CREATE TABLE IF NOT EXISTS webauthn_challenges (
  challenge_hash TEXT PRIMARY KEY,
  user_id        BIGINT REFERENCES users(id) ON DELETE CASCADE,
  purpose        TEXT NOT NULL CHECK (purpose IN ('register','login')),
  session_data   BYTEA NOT NULL,
  remember       BOOLEAN DEFAULT FALSE,
  expires_at     TIMESTAMPTZ NOT NULL,
  created_at     TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS webauthn_challenges_expiry_idx ON webauthn_challenges (expires_at);
CREATE TABLE IF NOT EXISTS auth_audit_log (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
  event       TEXT NOT NULL,
  ip_hint     TEXT,
  user_agent  TEXT,
  created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS auth_audit_user_idx ON auth_audit_log (user_id, created_at DESC);
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

CREATE TABLE IF NOT EXISTS bot_users (
  chat_id    BIGINT PRIMARY KEY,
  username   TEXT,
  first_name TEXT,
  authorized BOOLEAN DEFAULT FALSE,
  attempts   INT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT now(),
  last_seen  TIMESTAMPTZ DEFAULT now()
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

-- Platform-global NOXIOAI marketing queue. Telegram is human-approved before
-- publishing; Instagram rows remain ready for manual posting.
CREATE TABLE IF NOT EXISTS social_posts (
  id         BIGSERIAL PRIMARY KEY,
  platform   TEXT,
  caption    TEXT,
  image_url  TEXT,
  status     TEXT DEFAULT 'draft',          -- draft | approved | posted | rejected
  created_at TIMESTAMPTZ DEFAULT now(),
  posted_at  TIMESTAMPTZ
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

-- ── MADUSA persona agent (SPEC: approved 2026-07-20) ────────────────────────
-- Trend scout + content machine for the AI-automation niche: ingest YouTube
-- creators + Reddit/HN signals, score momentum, propose short-form video
-- ideas (the "MAP") for human approval before any rendering happens.
CREATE TABLE IF NOT EXISTS madusa_creators (
  id         BIGSERIAL PRIMARY KEY,
  platform   TEXT NOT NULL DEFAULT 'youtube',
  handle     TEXT NOT NULL UNIQUE,
  channel_id TEXT,
  title      TEXT,
  niche      TEXT DEFAULT 'ai-automation',
  active     BOOLEAN NOT NULL DEFAULT true,
  added_by   TEXT NOT NULL DEFAULT 'seed',
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS madusa_snapshots (
  id          BIGSERIAL PRIMARY KEY,
  creator_id  BIGINT NOT NULL REFERENCES madusa_creators(id) ON DELETE CASCADE,
  day         DATE NOT NULL DEFAULT current_date,
  subs        BIGINT,
  views       BIGINT,
  video_count INT,
  UNIQUE (creator_id, day)
);

CREATE TABLE IF NOT EXISTS madusa_videos (
  id           BIGSERIAL PRIMARY KEY,
  creator_id   BIGINT NOT NULL REFERENCES madusa_creators(id) ON DELETE CASCADE,
  video_id     TEXT NOT NULL UNIQUE,
  title        TEXT,
  published_at TIMESTAMPTZ,
  duration_s   INT,
  views        BIGINT,
  likes        BIGINT,
  comments     BIGINT,
  views_prev   BIGINT,
  fetched_prev TIMESTAMPTZ,
  fetched_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS madusa_signals (
  id           BIGSERIAL PRIMARY KEY,
  source       TEXT NOT NULL,
  title        TEXT NOT NULL,
  url          TEXT NOT NULL UNIQUE,
  score        INT,
  comments     INT,
  score_prev   INT,
  fetched_prev TIMESTAMPTZ,
  fetched_at   TIMESTAMPTZ DEFAULT now(),
  created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS madusa_trends (
  id            BIGSERIAL PRIMARY KEY,
  day           DATE NOT NULL DEFAULT current_date,
  topic         TEXT NOT NULL,
  stage         TEXT NOT NULL,
  velocity      REAL,
  evidence      TEXT,
  hook_patterns TEXT,
  created_at    TIMESTAMPTZ DEFAULT now()
);

-- status: proposed | approved | packed | rendering | delivered | rejected | failed
CREATE TABLE IF NOT EXISTS madusa_posts (
  id           BIGSERIAL PRIMARY KEY,
  trend_id     BIGINT REFERENCES madusa_trends(id),
  idea         TEXT NOT NULL,
  hook         TEXT,
  format       TEXT NOT NULL DEFAULT 'reel',
  storyboard   JSONB,
  caption_fa   TEXT,
  caption_en   TEXT,
  hashtags     TEXT,
  titles       JSONB,
  status       TEXT NOT NULL DEFAULT 'proposed',
  video_url    TEXT,
  image_url    TEXT,
  approved_at  TIMESTAMPTZ,
  delivered_at TIMESTAMPTZ,
  created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS madusa_renders (
  id          BIGSERIAL PRIMARY KEY,
  post_id     BIGINT REFERENCES madusa_posts(id),
  instance_id TEXT,
  instance_ip TEXT,
  status      TEXT NOT NULL DEFAULT 'creating',
  cost_hours  REAL,
  log         TEXT,
  started_at  TIMESTAMPTZ DEFAULT now(),
  finished_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_madusa_snapshots_creator ON madusa_snapshots(creator_id);
CREATE INDEX IF NOT EXISTS idx_madusa_videos_creator    ON madusa_videos(creator_id);
CREATE INDEX IF NOT EXISTS idx_madusa_posts_status      ON madusa_posts(status);
CREATE INDEX IF NOT EXISTS idx_madusa_posts_trend       ON madusa_posts(trend_id);
CREATE INDEX IF NOT EXISTS idx_madusa_renders_post      ON madusa_renders(post_id);

-- ── admin console persistent chat log (owner-facing JARVIS console only) ────
CREATE TABLE IF NOT EXISTS chat_messages (
  id         BIGSERIAL PRIMARY KEY,
  owner_id   BIGINT NOT NULL,
  role       TEXT NOT NULL,
  content    TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_owner_created ON chat_messages(owner_id, created_at DESC);
