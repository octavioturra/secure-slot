# Spec: Autenticação

## Fluxo Sérgio (receptor — conta corporativa BPS)

```
Browser → GET /api/v1/auth/login
       → redirect para Keycloak
       → Keycloak redireciona para Google OAuth (federado)
       → Google autentica com conta @bps.com.br
       → Keycloak valida domínio (hd=bps.com.br)
       → Keycloak emite tokens OIDC
       → callback → backend valida → emite JWT de sessão (15min)
       → frontend armazena em memória (não localStorage)
```

**Validação de domínio no backend:**
```go
// Rejeitar tokens de domínios não autorizados
if claims["hd"] != allowedDomain {
    return ErrUnauthorizedDomain
}
```

## Fluxo Nicoly (remetente — sem conta)

```
Nicoly abre link /s/:slotId
→ frontend requisita email
→ POST /api/v1/slots/:id/otp { email }
→ backend valida email contra slot.recipient_email
→ gera OTP 6 dígitos, TTL 5min, armazena hash no Redis
→ envia email com OTP via SMTP
→ Nicoly insere OTP
→ POST /api/v1/slots/:id/submit { otp, payload }
→ backend valida OTP, invalida após uso (one-time)
```

## Keycloak — configuração necessária

**Realm:** `secureslot`

**Identity Provider:** Google
- Client ID e Secret do Google Cloud Console
- Allowed domain: `bps.com.br` (mapper no Keycloak)
- Attribute mapper: `hd` → claim `hd` no token

**Client backend** (`secureslot-backend`):
- Tipo: confidential
- Grant types: authorization_code
- Redirect URIs: `http://localhost:3000/api/v1/auth/callback`

**Client frontend** (`secureslot-frontend`):
- Tipo: public
- Grant types: authorization_code + PKCE
- Redirect URIs: `http://localhost:5173/*`

## JWT de sessão (emitido pelo backend Go)

```go
type Claims struct {
    UserID  string `json:"sub"`
    Email   string `json:"email"`
    Domain  string `json:"hd"`
    jwt.RegisteredClaims
}
// Expiry: 15 minutos
// Algoritmo: RS256 (chave gerada no Vault)
```

## Passkey (WebAuthn) — registro

Ocorre após primeiro login de Sérgio, antes de criar o primeiro slot.

```
POST /api/v1/auth/passkey/register/begin
← { challenge, rp, user, ... }

navigator.credentials.create({ publicKey: options, extensions: { prf: ... } })

POST /api/v1/auth/passkey/register/complete { credential }
← 200 OK
```

O backend armazena: `credential_id`, `public_key_cose`, `sign_count`, `user_id`.
Não armazena material PRF — esse é derivado no browser sob demanda.

## Passkey (WebAuthn) — autenticação (para abrir slot)

```
POST /api/v1/auth/passkey/auth/begin
← { challenge, allowCredentials: [credentialId do usuário] }

navigator.credentials.get({ publicKey: options, extensions: { prf: { eval: { first: ... } } } })

POST /api/v1/auth/passkey/auth/complete { assertion }
← { verified: true }
```

Backend verifica assinatura e sign_count (anti-replay).
Não retorna token — serve apenas para confirmar identidade antes de expor wrapped_private_key.

## OTP

- 6 dígitos numéricos
- Gerado com `crypto/rand`
- Armazenado como bcrypt hash no Redis com TTL 5min
- One-time: invalidado após primeiro uso com sucesso
- Rate limit: 3 tentativas / 10min / IP (middleware)
- Email enviado via SMTP (MailHog em dev, SMTP real em prod)
