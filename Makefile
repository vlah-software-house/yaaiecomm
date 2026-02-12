.PHONY: help dev dev-down dev-db migrate migrate-down migrate-create seed sqlc templ generate lint test test-unit test-integration test-e2e build clean

# Default
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Development ──────────────────────────────────────────

dev: ## Start all dev services (postgres, mailpit, stripe-mock)
	docker compose up -d

dev-down: ## Stop all dev services
	docker compose down

dev-db: ## Start only postgres
	docker compose up -d postgres

# ── Database ─────────────────────────────────────────────

DB_URL ?= postgres://forge:forgedev@localhost:5432/forgecommerce?sslmode=disable

migrate: ## Run all pending migrations
	cd api && go run -tags migrate cmd/migrate/main.go -direction up -db "$(DB_URL)"

migrate-down: ## Rollback last migration
	cd api && go run -tags migrate cmd/migrate/main.go -direction down -steps 1 -db "$(DB_URL)"

migrate-create: ## Create new migration (usage: make migrate-create NAME=create_users)
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	@NUM=$$(printf "%03d" $$(($$(ls api/internal/database/migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' ') + 1))); \
	touch "api/internal/database/migrations/$${NUM}_$(NAME).up.sql"; \
	touch "api/internal/database/migrations/$${NUM}_$(NAME).down.sql"; \
	echo "Created migrations/$${NUM}_$(NAME).{up,down}.sql"

seed: ## Seed the database
	PGPASSWORD=forgedev psql -h localhost -U forge -d forgecommerce -f scripts/seed.sql

# ── Code Generation ──────────────────────────────────────

sqlc: ## Generate sqlc code
	cd api && sqlc generate

templ: ## Generate templ templates
	cd api && templ generate

generate: sqlc templ ## Generate all code (sqlc + templ)

# ── Quality ──────────────────────────────────────────────

lint: ## Run linters
	cd api && golangci-lint run ./...

fmt: ## Format Go code
	cd api && go fmt ./...
	cd api && templ fmt .

# ── Testing ──────────────────────────────────────────────

test: test-unit ## Run unit tests

test-unit: ## Run Go unit tests
	cd api && go test -race -short ./...

test-integration: ## Run Go integration tests (requires postgres)
	cd api && go test -race -run Integration ./...

test-e2e: ## Run Playwright E2E tests
	cd tests && npx playwright test

test-all: test-unit test-integration test-e2e ## Run all tests

# ── Build ────────────────────────────────────────────────

build-api: ## Build the Go API server
	cd api && go build -o bin/server cmd/server/main.go

build-storefront: ## Build the Nuxt storefront
	cd storefront && npm run build

build: build-api build-storefront ## Build everything

# ── Run ──────────────────────────────────────────────────

run-api: ## Run the API server
	cd api && go run cmd/server/main.go

run-api-watch: ## Run the API server with hot reload (air)
	cd api && air

run-storefront: ## Run the Nuxt storefront in dev mode
	cd storefront && npm run dev

# ── Cleanup ──────────────────────────────────────────────

clean: ## Clean build artifacts
	rm -rf api/bin/ api/tmp/
	rm -rf storefront/.nuxt/ storefront/.output/ storefront/dist/
	rm -rf test-results/ playwright-report/
