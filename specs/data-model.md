# Spec: Modelo de Dados

## PostgreSQL

### users
```sql
CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email         TEXT NOT NULL UNIQUE,
  domain        TEXT NOT NULL,  -- ex: bps.com.br
  display_name  TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ
);
```

### passkey_credentials
```sql
CREATE TABLE passkey_credentials (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_id TEXT NOT NULL UNIQUE,  -- base64url WebAuthn credential ID
  public_key    BYTEA NOT NULL,        -- COSE encoded public key
  sign_count    BIGINT NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at  TIMESTAMPTZ
);
CREATE INDEX ON passkey_credentials(user_id);
```

### slots
```sql
CREATE TABLE slots (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id            UUID NOT NULL REFERENCES users(id),
  label               TEXT NOT NULL,
  status              TEXT NOT NULL DEFAULT 'pending',  -- pending|filled|opened|expired
  recipient_email     TEXT NOT NULL,

  -- chaves (lado do receptor)
  public_key          TEXT NOT NULL,         -- base64 SPKI
  wrapped_private_key TEXT NOT NULL,         -- base64 AES-GCM wrapped PKCS8
  wrap_iv             TEXT NOT NULL,         -- base64 IV usado no wrap
  credential_id       TEXT NOT NULL,         -- WebAuthn credential ID do dono

  -- payload (preenchido pelo remetente)
  encrypted_aes_key   TEXT,                  -- null até submit
  payload_iv          TEXT,                  -- null até submit
  ciphertext          TEXT,                  -- null até submit

  -- envelope key reference (OCI Vault / HashiCorp Vault)
  vault_key_id        TEXT,                  -- ID da chave no Vault (se usado para envelope)

  ttl_hours           INT NOT NULL DEFAULT 72,
  expires_at          TIMESTAMPTZ NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  submitted_at        TIMESTAMPTZ,
  opened_at           TIMESTAMPTZ,
  deleted_at          TIMESTAMPTZ            -- soft delete para audit trail
);
CREATE INDEX ON slots(owner_id);
CREATE INDEX ON slots(status);
CREATE INDEX ON slots(expires_at);
```

### audit_log
```sql
CREATE TABLE audit_log (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  action     TEXT NOT NULL,     -- slot.created, slot.opened, slot.deleted, submit.received, otp.requested
  slot_id    UUID REFERENCES slots(id) ON DELETE SET NULL,
  actor      TEXT NOT NULL,     -- email do usuário ou 'system'
  ip         INET,
  user_agent TEXT,
  timestamp  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON audit_log(slot_id);
CREATE INDEX ON audit_log(actor);
CREATE INDEX ON audit_log(timestamp);
-- append-only: sem UPDATE, sem DELETE nesta tabela
-- enforçar via política no PostgreSQL em prod:
-- REVOKE UPDATE, DELETE ON audit_log FROM secureslot_app;
```

## Redis

### OTP
```
Key:   otp:{slot_id}:{email_hash}
Value: bcrypt hash do OTP
TTL:   300 (5 minutos)
```

### Rate limit OTP
```
Key:   ratelimit:otp:{ip}
Value: contador de tentativas
TTL:   600 (10 minutos)
```

### Session / JWT blacklist (logout)
```
Key:   jwt:blacklist:{jti}
Value: "1"
TTL:   tempo restante do JWT
```

## Tipos Go

```go
type SlotStatus string

const (
    SlotPending  SlotStatus = "pending"
    SlotFilled   SlotStatus = "filled"
    SlotOpened   SlotStatus = "opened"
    SlotExpired  SlotStatus = "expired"
)

type Slot struct {
    ID                UUID
    OwnerID           UUID
    Label             string
    Status            SlotStatus
    RecipientEmail    string
    PublicKey         string
    WrappedPrivateKey string
    WrapIV            string
    CredentialID      string
    EncryptedAESKey   *string
    PayloadIV         *string
    Ciphertext        *string
    VaultKeyID        *string
    TTLHours          int
    ExpiresAt         time.Time
    CreatedAt         time.Time
    SubmittedAt       *time.Time
    OpenedAt          *time.Time
    DeletedAt         *time.Time
}

type AuditEntry struct {
    ID        UUID
    Action    string
    SlotID    *UUID
    Actor     string
    IP        string
    UserAgent string
    Timestamp time.Time
}
```

## Migrações

Usar `golang-migrate`. Arquivos em `/backend/migrations/`.
Formato: `{timestamp}_{description}.up.sql` e `.down.sql`.
