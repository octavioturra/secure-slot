# SecureSlot

Transferência segura de dados sensíveis entre empresas, direto no browser.

Construído para o caso onde empresa A precisa enviar um lote de dados (ex: CPFs) para empresa B, com rastreabilidade, conformidade LGPD, e zero dependência de TI para uso rotineiro.

---

## O problema

Nicoly da Just precisa enviar 3 mil CPFs para Sérgio da BPS.

Hoje isso envolve Kleopatra, tickets de TI, troca manual de chaves PGP, e um processo que ninguém consegue fazer sozinho. Os dados trafegam por email ou drives sem garantia de quem pode abrir.

---

## A solução

Um slot de transferência criptografado end-to-end.

**Sérgio** (receptor) cria um slot no browser. Uma keypair RSA é gerada localmente — a chave privada nunca sai do dispositivo dele sem estar protegida pela sua passkey. Ele envia o link para Nicoly.

**Nicoly** (remetente) abre o link, se autentica com OTP no email corporativo, cola os dados e envia. A criptografia acontece no browser dela, antes dos dados tocarem qualquer servidor.

**Sérgio** abre o slot, usa a passkey para destrancar a chave privada, e os dados são decriptados — também no browser dele.

O servidor armazena apenas ciphertext opaco. Nunca vê os dados.

---

## Fluxo

```
Sérgio                          Servidor                        Nicoly
  │                                │                               │
  ├─ login (Google / Keycloak) ───▶│                               │
  ├─ gera keypair (browser) ──────▶│ armazena pubkey               │
  ├─ protege privkey (passkey)     │                               │
  ├─ cria slot ───────────────────▶│ envia link ──────────────────▶│
  │                                │                               ├─ OTP via email
  │                                │◀── dados encriptados (browser)┤
  │                                │    servidor nunca vê plaintext│
  ├─ abre slot ───────────────────▶│                               │
  ├─ passkey destrava privkey      │                               │
  ├─ decripta no browser ◀────────┤                               │
  │  (servidor não participa)      │                               │
```

---

## Segurança

- **Client-side encryption** — WebCrypto API nativa do browser (RSA-OAEP + AES-GCM)
- **Passkey** — chave privada protegida por WebAuthn PRF, nunca em localStorage
- **Servidor cego** — armazena apenas ciphertext + metadata
- **OTP one-time** — remetente autentica por email corporativo antes de ver o form
- **TTL configurável** — slot expira automaticamente, dados tornam-se irrecuperáveis
- **Audit log imutável** — quem acessou, quando, IP — sem conteúdo
- **CSP + SRI** — JS servido com hash verificável, mitigação de XSS

---

## Stack

| Camada | Tecnologia |
|---|---|
| Backend | Go 1.22+ |
| Frontend | React + Vite + TypeScript + Tailwind |
| Auth | Keycloak (OIDC + Google OAuth) |
| Crypto | WebCrypto API + WebAuthn (nativo no browser) |
| KMS | HashiCorp Vault (dev) · OCI Vault (prod) |
| Banco | PostgreSQL + Redis |
| Infra dev | Docker Compose |
| Infra prod | OCI VM + Docker Compose + nginx |

---

## Estrutura

```
├── CLAUDE.md              ← instruções para o Claude Code
├── PROVISIONING.md        ← setup do ambiente antes de codar
├── Makefile               ← comandos do projeto
├── specs/
│   ├── crypto.md          ← contratos WebCrypto + WebAuthn
│   ├── api.md             ← endpoints REST
│   ├── auth.md            ← fluxos de autenticação
│   ├── data-model.md      ← schema SQL + tipos Go
│   └── frontend.md        ← componentes + CSP/SRI
├── infra/
│   ├── docker-compose.yml ← ambiente local
│   ├── docker-compose.prod.yml
│   ├── seed.sh            ← configura Vault + Keycloak
│   └── keycloak/
├── backend/
└── frontend/
```

---

## Rodando localmente

```bash
# 1. sobe infra (postgres, redis, vault, keycloak, mailhog)
make infra-up

# 2. configura vault e keycloak
make infra-seed

# 3. cria backend/.env (ver PROVISIONING.md)

# 4. roda migrations
make migrate-up

# 5. inicia serviços (terminais separados)
make backend-dev
make frontend-dev
```

Acesse http://localhost:5173 · Login dev: `sergio@bps.com.br` / `dev123`

Emails (OTP): http://localhost:8025 (MailHog)

---

## Conformidade LGPD

- Dados nunca persistidos em plaintext
- Acesso restrito ao destinatário nomeado (OTP no email corporativo)
- TTL obrigatório — sem armazenamento indefinido
- Audit log de todos os acessos
- Deleção criptográfica: remover a envelope key torna o ciphertext irrecuperável
