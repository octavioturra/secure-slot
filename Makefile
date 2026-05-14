GO_VERSION  := 1.25.0
GO_INSTALL  := $(HOME)/.local/go
export PATH := $(GO_INSTALL)/bin:$(PATH)

.PHONY: infra-up infra-down infra-seed backend-dev frontend-dev test migrate-up migrate-down lint \
        ensure-go ensure-air ensure-migrate ensure-golangci-lint

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

# Ferramentas — instalam automaticamente se não estiverem no PATH
ensure-go:
	@if ! command -v go > /dev/null 2>&1; then \
		echo "Instalando Go $(GO_VERSION)..."; \
		OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
		ARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
		mkdir -p $(HOME)/.local; \
		curl -fsSL "https://go.dev/dl/go$(GO_VERSION).$$OS-$$ARCH.tar.gz" \
			| tar -C $(HOME)/.local -xz; \
		echo "Go $(GO_VERSION) instalado em $(GO_INSTALL)/bin"; \
		echo "Para usar fora do make, adicione ao seu shell:"; \
		echo "  export PATH=\$$PATH:$(GO_INSTALL)/bin"; \
	fi

ensure-air: ensure-go
	@command -v air > /dev/null 2>&1 || (echo "Instalando air..." && go install github.com/air-verse/air@latest)

ensure-migrate: ensure-go
	@command -v migrate > /dev/null 2>&1 || (echo "Instalando migrate..." && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)

ensure-golangci-lint: ensure-go
	@command -v golangci-lint > /dev/null 2>&1 || (echo "Instalando golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)

# Backend
backend-dev: ensure-air
	cd backend && air -c .air.toml

backend-build: ensure-go
	cd backend && go build -o bin/server ./cmd/server

# Migrations
migrate-up: ensure-migrate
	cd backend && migrate -path migrations -database "$$DATABASE_URL" up

migrate-down: ensure-migrate
	cd backend && migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-new: ensure-migrate
	@read -p "Nome da migration: " name; \
	cd backend && migrate create -ext sql -dir migrations -seq $$name

# Frontend
frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

# Testes
test: ensure-go
	cd backend && go test ./...

test-integration: ensure-go
	cd backend && go test -tags integration ./...

lint: ensure-golangci-lint
	cd backend && golangci-lint run
	cd frontend && npm run lint

# Setup inicial (primeira vez)
setup:
	@echo "Instalando ferramentas Go..."
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Instalando dependências frontend..."
	cd frontend && npm install
	@echo ""
	@echo "Pronto. Próximos passos:"
	@echo "  make infra-up"
	@echo "  make infra-seed"
	@echo "  make migrate-up"
	@echo "  make backend-dev   (em outro terminal)"
	@echo "  make frontend-dev  (em outro terminal)"
