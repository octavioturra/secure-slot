.PHONY: infra-up infra-down infra-seed backend-dev frontend-dev test migrate-up migrate-down lint

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
	cd backend && migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	cd backend && migrate -path migrations -database "$$DATABASE_URL" down 1

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
