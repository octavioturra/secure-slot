CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email         TEXT NOT NULL UNIQUE,
  domain        TEXT NOT NULL,
  display_name  TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ
);

CREATE TABLE passkey_credentials (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_id TEXT NOT NULL UNIQUE,
  public_key    BYTEA NOT NULL,
  sign_count    BIGINT NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at  TIMESTAMPTZ
);
CREATE INDEX ON passkey_credentials(user_id);

CREATE TABLE slots (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id            UUID NOT NULL REFERENCES users(id),
  label               TEXT NOT NULL,
  status              TEXT NOT NULL DEFAULT 'pending',
  recipient_email     TEXT NOT NULL,

  public_key          TEXT NOT NULL,
  wrapped_private_key TEXT NOT NULL,
  wrap_iv             TEXT NOT NULL,
  credential_id       TEXT NOT NULL,

  encrypted_aes_key   TEXT,
  payload_iv          TEXT,
  ciphertext          TEXT,

  vault_key_id        TEXT,

  ttl_hours           INT NOT NULL DEFAULT 72,
  expires_at          TIMESTAMPTZ NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  submitted_at        TIMESTAMPTZ,
  opened_at           TIMESTAMPTZ,
  deleted_at          TIMESTAMPTZ
);
CREATE INDEX ON slots(owner_id);
CREATE INDEX ON slots(status);
CREATE INDEX ON slots(expires_at);

CREATE TABLE audit_log (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  action     TEXT NOT NULL,
  slot_id    UUID REFERENCES slots(id) ON DELETE SET NULL,
  actor      TEXT NOT NULL,
  ip         INET,
  user_agent TEXT,
  timestamp  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON audit_log(slot_id);
CREATE INDEX ON audit_log(actor);
CREATE INDEX ON audit_log(timestamp);
