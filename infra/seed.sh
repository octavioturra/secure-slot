#!/usr/bin/env bash
# seed.sh — configura Vault e aguarda Postgres para dev local
# Dependências: docker, curl — nenhuma ferramenta local adicional

set -euo pipefail

VAULT_ADDR=http://localhost:8200
VAULT_TOKEN=dev-token
COMPOSE_FILE="infra/docker-compose.yml"

vault_api() {
  local method="$1"
  local path="$2"
  local data="${3:-}"

  if [[ -n "$data" ]]; then
    curl -sf -X "$method" "$VAULT_ADDR/v1/$path" \
      -H "X-Vault-Token: $VAULT_TOKEN" \
      -H "Content-Type: application/json" \
      -d "$data"
  else
    curl -sf -X "$method" "$VAULT_ADDR/v1/$path" \
      -H "X-Vault-Token: $VAULT_TOKEN"
  fi
}

echo "=== Aguardando Vault ficar pronto ==="
until vault_api GET sys/health > /dev/null 2>&1; do
  echo "  Vault não está pronto ainda, aguardando..."
  sleep 2
done
echo "  Vault pronto."

echo ""
echo "=== Configurando HashiCorp Vault ==="

# Habilita Transit Secrets Engine
if vault_api GET sys/mounts/transit > /dev/null 2>&1; then
  echo "  Transit já habilitado."
else
  vault_api POST sys/mounts/transit '{"type":"transit"}' > /dev/null
  echo "  Transit habilitado."
fi

# Cria chave de envelope (idempotente — ignora erro se já existe)
vault_api POST transit/keys/secureslot-envelope '{"type":"aes256-gcm96"}' > /dev/null 2>&1 || true
echo "  Chave secureslot-envelope criada (ou já existia)."

# Cria policy mínima para a aplicação
POLICY='{"policy":"path \"transit/encrypt/secureslot-envelope\" { capabilities = [\"update\"] }\npath \"transit/decrypt/secureslot-envelope\" { capabilities = [\"update\"] }\n"}'
vault_api PUT sys/policies/acl/secureslot-app "$POLICY" > /dev/null
echo "  Policy secureslot-app aplicada."

# Cria token com a policy e extrai com python3 (presente em qualquer macOS/Linux)
TOKEN_RESPONSE=$(vault_api POST auth/token/create '{"policies":["secureslot-app"],"ttl":"0"}')
VAULT_APP_TOKEN=$(echo "$TOKEN_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['auth']['client_token'])")

echo ""
echo "  VAULT_APP_TOKEN=$VAULT_APP_TOKEN"
echo "  Adicione ao backend/.env: VAULT_TOKEN=$VAULT_APP_TOKEN"

echo ""
echo "=== Aguardando PostgreSQL ==="
until docker compose -f "$COMPOSE_FILE" exec -T postgres pg_isready -U secureslot > /dev/null 2>&1; do
  echo "  PostgreSQL não está pronto ainda, aguardando..."
  sleep 2
done
echo "  PostgreSQL pronto."

echo ""
echo "=== Serviços disponíveis ==="
echo "  PostgreSQL:  localhost:5432  (secureslot/secureslot)"
echo "  Redis:       localhost:6379"
echo "  Vault UI:    http://localhost:8200  (token: dev-token)"
echo "  Keycloak:    http://localhost:8080  (admin/admin)"
echo "  MailHog:     http://localhost:8025"

echo ""
echo "=== Próximos passos ==="
echo "  1. Atualize backend/.env com VAULT_TOKEN=$VAULT_APP_TOKEN"
echo "  2. make backend-dev"
