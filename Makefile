GO_VERSION := 1.25.0
GO_INSTALL := $(HOME)/.local/go
export PATH := $(GO_INSTALL)/bin:$(HOME)/go/bin:$(PATH)

# Carrega backend/.env e exporta todas as variáveis para os sub-shells
ifneq (,$(wildcard backend/.env))
  include backend/.env
  export
endif

.PHONY: infra-up infra-down infra-seed infra-reset \
        backend-dev backend-build \
        migrate-up migrate-down migrate-new \
        frontend-dev frontend-build \
        test test-integration lint setup

# Infra local
infra-up:
	docker compose -f infra/docker-compose.yml up -d
	@echo "Aguardando serviços ficarem saudáveis..."
	@sleep 5
	@docker compose -f infra/docker-compose.yml ps

infra-down:
	docker compose -f infra/docker-compose.yml down

infra-seed:
	chmod +x infra/seed.sh && ./infra/seed.sh

infra-reset:
	docker compose -f infra/docker-compose.yml down -v
	docker compose -f infra/docker-compose.yml up -d

# Backend
backend-dev:
	cd backend && air -c .air.toml

backend-build:
	cd backend && go build -o bin/server ./cmd/server

# Migrations
migrate-up:
	cd backend && migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	cd backend && migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-new:
	@read -p "Nome da migration: " name; \
	cd backend && migrate create -ext sql -dir migrations -seq $$name

# Frontend
frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

# Testes
test:
	cd backend && go test ./...

test-integration:
	cd backend && go test -tags integration ./...

lint:
	cd backend && golangci-lint run
	cd frontend && npm run lint

# Setup inicial (primeira vez) — instala Go e todas as ferramentas
setup:
	@echo "=== Instalando Go $(GO_VERSION) ==="
	@if ! command -v go > /dev/null 2>&1; then \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
		mkdir -p $(HOME)/.local; \
		curl -fsSL "https://go.dev/dl/go$(GO_VERSION).$$OS-$$ARCH.tar.gz" \
			| tar -C $(HOME)/.local -xz; \
		echo "Go $(GO_VERSION) instalado em $(GO_INSTALL)/bin"; \
	else \
		echo "Go já instalado: $$(go version)"; \
	fi
	@echo ""
	@echo "=== Instalando ferramentas Go ==="
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo ""
	@echo "=== Instalando dependências frontend ==="
	cd frontend && npm install
	@echo ""
	@echo "=== Adicionando Go ao PATH do shell ==="
	@SHELL_RC=""; \
	if [ -f "$(HOME)/.zshrc" ]; then SHELL_RC="$(HOME)/.zshrc"; \
	elif [ -f "$(HOME)/.bashrc" ]; then SHELL_RC="$(HOME)/.bashrc"; \
	fi; \
	if [ -n "$$SHELL_RC" ]; then \
		if ! grep -q "$(GO_INSTALL)/bin" "$$SHELL_RC"; then \
			echo 'export PATH="$$PATH:$(GO_INSTALL)/bin:$(HOME)/go/bin"' >> "$$SHELL_RC"; \
			echo "PATH adicionado em $$SHELL_RC — reabra o terminal ou: source $$SHELL_RC"; \
		else \
			echo "PATH já configurado em $$SHELL_RC"; \
		fi; \
	fi
	@echo ""
	@echo "=== Pronto. Próximos passos ==="
	@echo "  1. make infra-up"
	@echo "  2. make infra-seed"
	@echo "  3. Atualize backend/.env com o VAULT_TOKEN impresso pelo seed"
	@echo "  4. make migrate-up"
	@echo "  5. make backend-dev   (em outro terminal)"
	@echo "  6. make frontend-dev  (em outro terminal)"
