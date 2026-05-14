# SecureSlot — Claude Code System Prompt

## O que é esse projeto

SecureSlot é uma aplicação browser-first de transferência segura de dados sensíveis (ex: CPFs) entre empresas.
Criptografia acontece no cliente (WebCrypto API). O servidor nunca vê plaintext.
Desenhado para conformidade LGPD, UX enterprise, zero dependência de TI para uso rotineiro.

## Personas

- **Sérgio (receptor)**: autentica via Google OAuth (Workspace corporativo), cria slots, configura TTL
- **Nicoly (remetente)**: recebe link + OTP, insere dados, envia — sem conta, sem instalação

## Fluxo principal

```
Sérgio faz login (Google OAuth, domínio BPS)
  → cria slot (TTL, label)
  → keypair gerada no browser (WebCrypto RSA-OAEP)
  → privkey protegida por passkey (WebAuthn)
  → pubkey enviada ao servidor
  → servidor retorna link único para Nicoly

Nicoly abre link
  → servidor envia OTP para email corporativo dela
  → Nicoly autentica via OTP
  → dados encriptados localmente (AES-GCM, chave encriptada com RSA pubkey do Sérgio)
  → ciphertext enviado ao servidor (servidor nunca vê plaintext)

Sérgio abre slot
  → usa passkey para desencriptar privkey local
  → desencripta AES key
  → desencripta dados — tudo no browser
  → slot expira conforme TTL configurado
```

## Stack

- **Backend**: Go 1.22+
- **Frontend**: React + Vite + TypeScript + Tailwind
- **Auth**: Keycloak (OIDC/Google OAuth federation)
- **Crypto**: WebCrypto API nativa (RSA-OAEP + AES-GCM), WebAuthn para passkey
- **KMS (envelope key)**: HashiCorp Vault (dev local) / OCI Vault (prod)
- **DB**: PostgreSQL
- **Infra local**: Docker Compose
- **Infra prod**: OCI + OKE (Kubernetes) + Terraform

## Princípios de código

- Don't Repeat Yourself
- Keep It Simple
- You Ain't Gonna Need It
- Clean Code: nomes descritivos, funções pequenas, sem comentários óbvios
- Coesão: cada pacote/componente tem responsabilidade única
- Go: erros explícitos, sem panic em produção, interfaces para dependências externas
- React: componentes funcionais, hooks, sem class components

## Segurança — regras não negociáveis

- Servidor NUNCA recebe ou loga plaintext
- Chave privada NUNCA sai do browser sem estar encriptada pela passkey
- Todos os endpoints autenticados com JWT (15min expiry)
- CSP header: `default-src 'none'` com allowlist mínima
- SRI hash em todos os assets JS/CSS servidos
- HSTS obrigatório
- Logs de auditoria: append-only, sem conteúdo — só metadata (quem, quando, IP, ação)

## Interfaces obrigatórias (Go)

```go
// Todas as dependências externas por interface — facilita mock em dev/test
type KeyManager interface {
    Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
    Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

type SlotRepository interface {
    Create(ctx context.Context, slot Slot) error
    Get(ctx context.Context, id string) (*Slot, error)
    Delete(ctx context.Context, id string) error
    ListByOwner(ctx context.Context, ownerID string) ([]Slot, error)
}

type AuditLogger interface {
    Log(ctx context.Context, entry AuditEntry) error
}

type Notifier interface {
    SendOTP(ctx context.Context, email, otp string) error
}
```

## Estrutura de diretórios esperada

```
/
├── CLAUDE.md               ← este arquivo
├── specs/
│   ├── crypto.md           ← spec da camada crypto
│   ├── api.md              ← spec dos endpoints REST
│   ├── auth.md             ← spec do fluxo de autenticação
│   ├── frontend.md         ← spec dos componentes React
│   └── data-model.md       ← spec do modelo de dados
├── infra/
│   ├── docker-compose.yml  ← ambiente local completo
│   ├── terraform/          ← OCI prod
│   └── k8s/                ← manifests Kubernetes
├── backend/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── slot/
│   │   ├── auth/
│   │   ├── audit/
│   │   ├── notify/
│   │   └── keymgr/
│   └── pkg/
├── frontend/
│   ├── src/
│   │   ├── crypto/         ← WebCrypto + WebAuthn wrappers
│   │   ├── components/
│   │   ├── pages/
│   │   └── api/
│   └── vite.config.ts
└── Makefile
```

## Variáveis de ambiente (dev)

```
# Backend
DATABASE_URL=postgres://secureslot:secureslot@localhost:5432/secureslot
VAULT_ADDR=http://localhost:8200
VAULT_TOKEN=dev-token
KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=secureslot
KEYCLOAK_CLIENT_ID=secureslot-backend
GOOGLE_ALLOWED_DOMAIN=bps.com.br
JWT_SECRET=dev-secret-change-in-prod
OTP_EMAIL_FROM=noreply@secureslot.dev
SLOT_DEFAULT_TTL_HOURS=72

# Frontend
VITE_API_URL=http://localhost:3000
VITE_KEYCLOAK_URL=http://localhost:8080
VITE_KEYCLOAK_REALM=secureslot
VITE_KEYCLOAK_CLIENT_ID=secureslot-frontend
```

## Como rodar localmente

```bash
make infra-up      # sobe docker-compose (postgres, vault, keycloak, mailhog)
make infra-seed    # configura keycloak realm + vault secrets engine
make backend-dev   # go run com hot reload (air)
make frontend-dev  # vite dev server
make test          # testes unitários + integração
```

## Referências das specs

Antes de implementar qualquer módulo, leia a spec correspondente em `/specs`.
Cada spec tem: contrato, tipos, casos de erro, e exemplo de uso.
