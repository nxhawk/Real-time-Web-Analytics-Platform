# Pulse Analytics — single entry point for every development command.
# Run `make` or `make help` for the list.

SHELL := /bin/bash
.DEFAULT_GOAL := help
.ONESHELL:

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------

BACKEND_DIR   := backend
FRONTEND_DIR  := frontend
BIN_DIR       := $(BACKEND_DIR)/bin
COMPOSE       := docker compose
COMPOSE_BENCH := docker compose -f docker-compose.yml -f docker-compose.bench.yml

MODULE     := github.com/nxhawk/pulse-analytics/backend
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w \
	-X $(MODULE)/internal/version.Tag=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.BuildTime=$(BUILD_TIME)

SERVICES := ingest-api analytics-api

# Seeder / benchmark knobs, overridable: `make seed N=10000000`
N     ?= 1000000
DAYS  ?= 30
SITES ?= 1

export GIT_COMMIT := $(COMMIT)
export GIT_TAG    := $(VERSION)

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

.PHONY: help
help: ## Show this help
	@echo "Pulse Analytics — make targets"
	@echo
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Docker stack
# ---------------------------------------------------------------------------

.PHONY: up
up: ## Start the stack (ClickHouse + APIs) and wait until it is healthy
	$(COMPOSE) up -d --build
	@$(MAKE) --no-print-directory health

.PHONY: down
down: ## Stop the stack, keep the volumes
	$(COMPOSE) down

.PHONY: down-app
down-app: ## Stop only the API containers, keep ClickHouse running
	$(COMPOSE) stop ingest-api analytics-api

.PHONY: nuke
nuke: ## Stop the stack and delete the volumes (all local data is lost)
	$(COMPOSE) down -v

.PHONY: ps
ps: ## List services and their health
	$(COMPOSE) ps

.PHONY: logs
logs: ## Follow logs of every service (make logs S=ingest-api for one)
	$(COMPOSE) logs -f --tail=100 $(S)

.PHONY: health
health: ## Poll /healthz on both APIs until they answer
	@for port in 8080 8081; do \
		printf "waiting for :%s " $$port; \
		for i in $$(seq 1 30); do \
			if curl -fsS "http://localhost:$$port/healthz" >/dev/null 2>&1; then \
				echo "ok"; break; \
			fi; \
			printf "."; sleep 1; \
			if [ $$i -eq 30 ]; then echo " TIMEOUT"; exit 1; fi; \
		done; \
	done
	@echo "stack is up: ingest http://localhost:8080 · analytics http://localhost:8081"

# ---------------------------------------------------------------------------
# Go
# ---------------------------------------------------------------------------

.PHONY: deps
deps: ## Resolve and download Go dependencies (run once after cloning)
	cd $(BACKEND_DIR) && go mod tidy && go mod download && go mod verify

.PHONY: build
build: ## Build every backend binary into backend/bin/
	cd $(BACKEND_DIR) && for svc in $(SERVICES); do \
		echo "building $$svc"; \
		CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$$svc ./cmd/$$svc || exit 1; \
	done

.PHONY: run
run: ## Run the ingest API locally against the containerised ClickHouse
	cd $(BACKEND_DIR) && go run ./cmd/ingest-api

.PHONY: run-analytics
run-analytics: ## Run the analytics API locally
	cd $(BACKEND_DIR) && go run ./cmd/analytics-api

.PHONY: test
test: ## Run unit tests with the race detector
	cd $(BACKEND_DIR) && go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	cd $(BACKEND_DIR) && go tool cover -func=coverage.out | tail -1

.PHONY: test-int
test-int: ## Run integration tests (starts real containers via testcontainers)
	cd $(BACKEND_DIR) && go test -race -count=1 -tags=integration ./test/integration/...

.PHONY: cover
cover: test ## Run tests and open the HTML coverage report
	cd $(BACKEND_DIR) && go tool cover -html=coverage.out -o coverage.html
	@echo "report: $(BACKEND_DIR)/coverage.html"

.PHONY: lint
lint: ## Run golangci-lint
	cd $(BACKEND_DIR) && golangci-lint run ./...

.PHONY: fmt
fmt: ## Format Go code (gofmt + goimports when available)
	cd $(BACKEND_DIR) && gofmt -w -s $$(find . -name '*.go' -not -path './bin/*')
	@command -v goimports >/dev/null 2>&1 \
		&& (cd $(BACKEND_DIR) && goimports -local $(MODULE) -w .) \
		|| echo "goimports not installed, skipped (go install golang.org/x/tools/cmd/goimports@latest)"

.PHONY: vet
vet: ## Run go vet
	cd $(BACKEND_DIR) && go vet ./...

.PHONY: check
check: fmt vet lint test ## Everything CI runs, in one command

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations
	cd $(BACKEND_DIR) && go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	cd $(BACKEND_DIR) && go run ./cmd/migrate down

.PHONY: migrate-status
migrate-status: ## Show which migrations have been applied
	cd $(BACKEND_DIR) && go run ./cmd/migrate status

.PHONY: migrate-create
migrate-create: ## Create a migration: make migrate-create NAME=add_events_codec
	@test -n "$(NAME)" || (echo "NAME is required: make migrate-create NAME=..." && exit 1)
	cd $(BACKEND_DIR) && go run ./cmd/migrate create $(NAME)

.PHONY: ch-cli
ch-cli: ## Open a clickhouse-client shell inside the container
	$(COMPOSE) exec clickhouse clickhouse-client \
		--user pulse --password $${CLICKHOUSE_PASSWORD:-pulse} --database analytics

# ---------------------------------------------------------------------------
# Data & benchmarks
# ---------------------------------------------------------------------------

.PHONY: seed
seed: ## Generate synthetic events: make seed N=10000000
	cd $(BACKEND_DIR) && go run ./cmd/seeder -n $(N) -days $(DAYS) -sites $(SITES)

.PHONY: bench
bench: ## Run the ClickHouse vs PostgreSQL benchmark suite
	$(COMPOSE_BENCH) up -d postgres
	cd $(BACKEND_DIR) && go run ../loadtest/bench/run_bench.go

.PHONY: loadtest
loadtest: ## Run the k6 ingest load test
	k6 run loadtest/k6/ingest.js

# ---------------------------------------------------------------------------
# Frontend
# ---------------------------------------------------------------------------

.PHONY: web-dev
web-dev: ## Run the Next.js dashboard in development mode
	cd $(FRONTEND_DIR) && npm run dev

.PHONY: web-build
web-build: ## Build the Next.js dashboard
	cd $(FRONTEND_DIR) && npm run build

# ---------------------------------------------------------------------------
# Housekeeping
# ---------------------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts and local data
	rm -rf $(BIN_DIR) $(BACKEND_DIR)/coverage.out $(BACKEND_DIR)/coverage.html
	rm -rf data/ $(FRONTEND_DIR)/.next

.PHONY: tools
tools: ## Install the development tools used by make lint / make fmt
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
