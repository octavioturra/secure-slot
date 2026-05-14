#!/usr/bin/env bash
# seed.sh — configura Vault e Keycloak para dev local
# Rodar após: docker compose up -d && docker compose ps (todos healthy)

set -euo pipefail

echo "=== Configurando HashiCorp Vault ==="

export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=dev-token

# Habilita Transit Secrets Engine (equivalente ao KMS)
vault secrets enable transit 2>/dev/null || echo "transit já habilitado"

# Cria chave de envelope para o SecureSlot
vault write -f transit/keys/secureslot-envelope type=aes256-gcm96

# Cria policy mínima para a aplicação
vault policy write secureslot-app - <<EOF
path "transit/encrypt/secureslot-envelope" { capabilities = ["update"] }
path "transit/decrypt/secureslot-envelope" { capabilities = ["update"] }
EOF

# Cria token com a policy (usa em DATABASE_URL no backend)
VAULT_APP_TOKEN=$(vault token create -policy=secureslot-app -ttl=0 -format=json | jq -r '.auth.client_token')
echo "VAULT_APP_TOKEN=$VAULT_APP_TOKEN"
echo "Adicione ao .env do backend: VAULT_TOKEN=$VAULT_APP_TOKEN"

echo ""
echo "=== Configurando banco de dados ==="

# Aguarda postgres estar pronto
until pg_isready -h localhost -U secureslot 2>/dev/null; do
  echo "Aguardando PostgreSQL..."
  sleep 2
done

echo "PostgreSQL pronto."
echo "Execute as migrations: cd backend && make migrate-up"

echo ""
echo "=== Serviços disponíveis ==="
echo "PostgreSQL:  localhost:5432  (secureslot/secureslot)"
echo "Redis:       localhost:6379"
echo "Vault UI:    http://localhost:8200  (token: dev-token)"
echo "Keycloak:    http://localhost:8080  (admin/admin)"
echo "MailHog:     http://localhost:8025"
echo ""
echo "=== Próximos passos ==="
echo "1. Configure o Google OAuth no Keycloak (Identity Provider > Google)"
echo "   Client ID e Secret: Google Cloud Console > APIs > Credentials"
echo "2. Adicione GOOGLE_ALLOWED_DOMAIN=bps.com.br no .env"
echo "3. make backend-dev"
echo "4. make frontend-dev"
