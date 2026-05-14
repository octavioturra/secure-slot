# Guia de Provisionamento

Execute isso **antes** de abrir o Claude Code.
O objetivo é ter o ambiente local rodando e o OCI configurado.

---

## 1. Pré-requisitos locais

```bash
# Go 1.22+
go version

# Node 20+
node --version

# Docker Desktop (ou Orbstack no Mac)
docker --version

# Ferramentas Go
make setup
```

---

## 2. Ambiente local (dev)

```bash
# Sobe todos os serviços
make infra-up

# Verifica saúde (aguarde todos "healthy")
docker compose -f infra/docker-compose.yml ps

# Configura Vault e semeia dados de dev
make infra-seed

# Guarde o VAULT_APP_TOKEN impresso pelo seed.sh
```

**Serviços após `infra-up`:**
- PostgreSQL → `localhost:5432`
- Redis → `localhost:6379`
- Vault UI → http://localhost:8200 (token: `dev-token`)
- Keycloak → http://localhost:8080 (admin/admin, realm `secureslot` já importado)
- MailHog → http://localhost:8025 (UI para ver emails)

**Usuário de dev já criado no Keycloak:**
- Email: `sergio@bps.com.br`
- Senha: `dev123`

---

## 3. Arquivo .env (backend)

Crie `backend/.env`:

```env
DATABASE_URL=postgres://secureslot:secureslot@localhost:5432/secureslot?sslmode=disable
REDIS_URL=redis://localhost:6379

VAULT_ADDR=http://localhost:8200
VAULT_TOKEN=<VAULT_APP_TOKEN do passo 2>

KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=secureslot
KEYCLOAK_CLIENT_ID=secureslot-backend
KEYCLOAK_CLIENT_SECRET=dev-backend-secret

GOOGLE_ALLOWED_DOMAIN=bps.com.br

JWT_SECRET=dev-secret-troque-em-prod
JWT_EXPIRY_MINUTES=15

OTP_EMAIL_FROM=noreply@secureslot.dev
SMTP_HOST=localhost
SMTP_PORT=1025
SMTP_USER=
SMTP_PASS=

SLOT_DEFAULT_TTL_HOURS=72
MAX_PAYLOAD_BYTES=10485760

PORT=3000
ENV=development
```

---

## 4. Google OAuth (opcional em dev, obrigatório em prod)

Em dev, o Keycloak já tem o usuário `sergio@bps.com.br` com senha. Google OAuth é opcional.

Para habilitar Google OAuth:
1. Google Cloud Console → Criar projeto → APIs & Services → Credentials
2. OAuth 2.0 Client ID → Web application
3. Authorized redirect URIs: `http://localhost:8080/realms/secureslot/broker/google/endpoint`
4. Copiar Client ID e Secret
5. Keycloak → realm secureslot → Identity Providers → Google → habilitar e preencher
6. Ou edite `infra/keycloak/realm-export.json` com os valores e rode `make infra-reset`

---

## 5. OCI (produção) — provisionar depois do MVP

Pré-requisitos:
- Conta OCI criada (free tier, região `sa-saopaulo-1`)
- Conta convertida para PAYG (necessário para OKE)
- OCI CLI instalado e configurado (`oci setup config`)

```bash
# Instala OCI CLI
brew install oci-cli  # ou pip install oci-cli

# Configura credenciais
oci setup config
# Pede: tenancy OCID, user OCID, region, gera chave RSA
# Cole a public key no OCI Console: Identity > Users > API Keys

# Verifica
oci iam user list
```

Terraform para OCI está em `infra/terraform/` — **a ser gerado pelo Claude Code** após o MVP local estar funcional.

---

## 6. Verificação final antes de codar

```bash
# Backend conecta no banco?
cd backend && go run ./cmd/healthcheck

# Migrations rodam?
make migrate-up

# Frontend builda?
cd frontend && npm run build

# Tudo verde:
make backend-dev   # terminal 1
make frontend-dev  # terminal 2
# Acesse http://localhost:5173
# Login: sergio@bps.com.br / dev123
```

---

## Ordem de desenvolvimento recomendada

1. **Estrutura base Go** — cmd/server, router, middleware de auth
2. **Migrations** — schema do banco
3. **Keymgr package** — interface + implementação Vault
4. **Slot CRUD** — repositório + handlers
5. **Auth** — JWT middleware, OIDC callback, passkey endpoints
6. **OTP flow** — geração, envio, validação
7. **Frontend base** — Vite + React + roteamento
8. **CryptoProvider** — WebCrypto + WebAuthn wrappers
9. **Páginas** — Dashboard, CreateSlot, SubmitPage, OpenSlot
10. **CSP + SRI** — build pipeline + headers Go
11. **Testes** — unitários Go, testes de crypto no browser
12. **Infra OCI** — Terraform + Kubernetes manifests
