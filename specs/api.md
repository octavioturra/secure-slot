# Spec: API REST

## Convenções

- Prefixo: `/api/v1`
- Auth: Bearer JWT em todos os endpoints exceto `/auth/*` e `/slots/:id/submit`
- Erros: `{ "error": "mensagem humana", "code": "SNAKE_CASE_CODE" }`
- Datas: ISO 8601 UTC
- IDs: UUIDv4

---

## Auth

### `GET /api/v1/auth/login`
Redireciona para Keycloak (Google OAuth). Não requer corpo.

### `GET /api/v1/auth/callback`
Callback OIDC. Retorna JWT de sessão.

**Response 200:**
```json
{ "token": "eyJ...", "expires_at": "2024-01-01T00:15:00Z" }
```

### `POST /api/v1/auth/refresh`
Requer: Bearer token válido ou refresh token em httpOnly cookie.

---

## Slots

### `POST /api/v1/slots`
Cria um slot. Requer auth (Sérgio).

**Request:**
```json
{
  "label": "CPFs Jan/2025 — Just → BPS",
  "ttl_hours": 72,
  "recipient_email": "nicoly@just.com.br",
  "public_key": "base64-spki...",
  "wrapped_private_key": "base64...",
  "wrap_iv": "base64...",
  "credential_id": "base64-webauthn-id..."
}
```

**Response 201:**
```json
{
  "id": "uuid",
  "submit_url": "https://app.secureslot.dev/s/uuid",
  "submit_token": "token-para-nicoly",
  "expires_at": "2024-01-04T00:00:00Z"
}
```

**Efeito colateral:** envia email para `recipient_email` com `submit_url`.

---

### `GET /api/v1/slots`
Lista slots do usuário autenticado.

**Response 200:**
```json
{
  "slots": [
    {
      "id": "uuid",
      "label": "...",
      "status": "pending | filled | opened | expired",
      "created_at": "...",
      "expires_at": "...",
      "submitted_at": "...",  // null se pending
      "opened_at": "..."      // null se não aberto
    }
  ]
}
```

---

### `GET /api/v1/slots/:id`
Detalhes de um slot. Requer auth, dono do slot.

**Response 200:**
```json
{
  "id": "uuid",
  "label": "...",
  "status": "...",
  "public_key": "base64-spki...",
  "wrapped_private_key": "base64...",
  "wrap_iv": "base64...",
  "credential_id": "base64...",
  "encrypted_payload": {         // null se status=pending
    "encrypted_aes_key": "base64...",
    "iv": "base64...",
    "ciphertext": "base64..."
  },
  "expires_at": "...",
  "created_at": "..."
}
```

---

### `DELETE /api/v1/slots/:id`
Deleta slot e todos os dados associados. Requer auth, dono.

**Response 204** — sem corpo.

Internamente: deleta envelope key do Vault antes de deletar do banco. Garante irrecuperabilidade.

---

## Submit (Nicoly — sem auth de conta)

### `POST /api/v1/slots/:id/otp`
Nicoly solicita OTP. Não requer auth de conta.

**Request:**
```json
{ "email": "nicoly@just.com.br" }
```

**Validação:** email deve bater com `recipient_email` do slot.

**Response 200:**
```json
{ "message": "OTP enviado" }
```

Rate limit: 3 tentativas por 10 minutos por IP.

---

### `POST /api/v1/slots/:id/submit`
Envia dados encriptados. Requer OTP válido (header `X-OTP-Token`).

**Request:**
```json
{
  "otp": "123456",
  "payload": {
    "encrypted_aes_key": "base64...",
    "iv": "base64...",
    "ciphertext": "base64..."
  }
}
```

**Response 200:**
```json
{ "message": "Dados recebidos com sucesso" }
```

**Validações:**
- Slot deve estar em status `pending`
- Slot não pode estar expirado
- OTP válido e não expirado (5 min de validade)
- Payload não pode exceder 10MB (proteção contra abuse)

---

## Audit

### `GET /api/v1/audit`
Log de auditoria do usuário autenticado. Paginado.

**Query params:** `page`, `per_page` (max 100), `from`, `to`

**Response 200:**
```json
{
  "entries": [
    {
      "id": "uuid",
      "action": "slot.created | slot.opened | slot.deleted | submit.received | otp.requested",
      "slot_id": "uuid",
      "actor": "sergio@bps.com.br | nicoly@just.com.br | system",
      "ip": "1.2.3.4",
      "timestamp": "..."
    }
  ],
  "total": 42,
  "page": 1
}
```

---

## Códigos de erro

| Code | HTTP | Descrição |
|---|---|---|
| `SLOT_NOT_FOUND` | 404 | Slot inexistente ou não pertence ao usuário |
| `SLOT_EXPIRED` | 410 | TTL expirado |
| `SLOT_ALREADY_FILLED` | 409 | Submit já realizado |
| `INVALID_OTP` | 401 | OTP inválido ou expirado |
| `WRONG_RECIPIENT` | 403 | Email não bate com slot |
| `RATE_LIMITED` | 429 | Muitas tentativas |
| `PAYLOAD_TOO_LARGE` | 413 | Payload > 10MB |
| `UNAUTHORIZED` | 401 | JWT inválido ou expirado |
| `FORBIDDEN` | 403 | Recurso não pertence ao usuário |
