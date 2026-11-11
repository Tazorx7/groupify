SHELL := /bin/bash
.DEFAULT_GOAL := help

# Container engine: podman preferred (rootless), docker as fallback.
ENGINE := $(shell command -v podman >/dev/null 2>&1 && echo podman || echo docker)

DB_CONTAINER := groupify-db
DB_IMAGE     := docker.io/library/postgres:18-alpine
DB_PORT      := 5433
DB_USER      := groupify
DB_PASS      := groupify
DB_NAME      := groupify
DB_TEST_NAME := groupify_test
DB_VOLUME    := groupify-pgdata

BACKEND := backend

.PHONY: help env db-up db-down db-reset db-logs psql psql-test \
        api seed build web image e2e test test-integration race fmt vet tidy check clean

help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

env: ## Create .env from .env.example if missing
	@test -f .env || (cp .env.example .env && echo "created .env")

db-up: env ## Start the PostgreSQL container (port 5433) and ensure databases exist
	@if $(ENGINE) container exists $(DB_CONTAINER) >/dev/null 2>&1; then \
		echo "starting existing container $(DB_CONTAINER)"; \
		$(ENGINE) start $(DB_CONTAINER) >/dev/null; \
	else \
		echo "creating container $(DB_CONTAINER)"; \
		$(ENGINE) run -d --name $(DB_CONTAINER) \
			-p $(DB_PORT):5432 \
			-e POSTGRES_USER=$(DB_USER) \
			-e POSTGRES_PASSWORD=$(DB_PASS) \
			-e POSTGRES_DB=$(DB_NAME) \
			-v $(DB_VOLUME):/var/lib/postgresql \
			--health-cmd "pg_isready -U $(DB_USER) -d $(DB_NAME)" \
			--health-interval 2s --health-timeout 3s --health-retries 30 \
			$(DB_IMAGE) >/dev/null; \
	fi
	@echo -n "waiting for postgres"; \
	for i in $$(seq 1 60); do \
		if $(ENGINE) exec $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) -tAc 'SELECT 1' >/dev/null 2>&1; then echo " ready"; break; fi; \
		echo -n "."; sleep 1; \
	done
	@$(ENGINE) exec $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) -tAc \
		"SELECT 1 FROM pg_database WHERE datname='$(DB_TEST_NAME)'" | grep -q 1 \
		|| $(ENGINE) exec $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) -c \
			"CREATE DATABASE $(DB_TEST_NAME)" >/dev/null
	@echo "postgres: postgres://$(DB_USER):$(DB_PASS)@127.0.0.1:$(DB_PORT)/$(DB_NAME)"

db-down: ## Stop the PostgreSQL container (keeps data)
	-@$(ENGINE) stop $(DB_CONTAINER) >/dev/null 2>&1 && echo "stopped $(DB_CONTAINER)"

db-reset: ## Destroy the PostgreSQL container and its volume
	-@$(ENGINE) rm -f -v $(DB_CONTAINER) >/dev/null 2>&1
	-@$(ENGINE) volume rm -f $(DB_VOLUME) >/dev/null 2>&1
	@echo "database destroyed"

db-logs: ## Tail PostgreSQL logs
	@$(ENGINE) logs -f $(DB_CONTAINER)

psql: ## Open psql on the dev database
	@$(ENGINE) exec -it $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME)

psql-test: ## Open psql on the test database
	@$(ENGINE) exec -it $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_TEST_NAME)

api: env ## Run the API (auto-migrates)
	cd $(BACKEND) && go run ./cmd/api

seed: env ## Create/update the bootstrap admin from ADMIN_EMAIL / ADMIN_PASSWORD
	cd $(BACKEND) && go run ./cmd/seed

build: ## Build the API binary
	cd $(BACKEND) && go build -trimpath -o bin/groupify-api ./cmd/api

web: ## Build the SPA into frontend/dist
	cd frontend && npm ci && npm run build

image: ## Build the single-origin container (API + SPA)
	$(ENGINE) build -t groupify -f Dockerfile .

test: ## Unit tests (no database required)
	cd $(BACKEND) && go test ./... -count=1

test-integration: ## Full test suite including database-backed integration tests
	cd $(BACKEND) && TEST_DATABASE_URL=$$(grep -E '^TEST_DATABASE_URL=' ../.env 2>/dev/null | cut -d= -f2- || echo "postgres://groupify:groupify@127.0.0.1:5433/groupify_test?sslmode=disable") \
		go test ./... -count=1 -p 1

race: ## Tests with the race detector
	cd $(BACKEND) && go test ./... -race -count=1 -p 1

fmt: ## Format Go code
	cd $(BACKEND) && gofmt -w . && go vet ./...

tidy: ## Tidy modules
	cd $(BACKEND) && go mod tidy

e2e: db-up ## Browser suite against a throwaway stack (builds both halves)
	./scripts/e2e.sh

check: fmt test web ## Quick local gate (Go + SPA build)
