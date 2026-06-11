.PHONY: help
.PHONY: compose-up compose-down compose-logs compose-ps
.PHONY: install-tools install-tools-latest install-go-tools install-codegen-python-deps
.PHONY: merge-schemas openapi-bundle oapi-codegen validate-openapi openapi sqlc mockery generate
.PHONY: migrate-up migrate-down migrate-status migrate-create
.PHONY: build run clean
.PHONY: test test-race tese-race test-unit test-integration test-e2e
.PHONY: lint lint-fix deps-audit fmt mod-tidy

SHELL := /usr/bin/env bash

ENV_FILE ?= ../.env
ifeq ($(wildcard $(ENV_FILE)),)
ENV_FILE := ../.env.example
endif

ifneq ($(wildcard $(ENV_FILE)),)
include $(ENV_FILE)
export
endif

APP_BIN := bin/videolibrary
GO_PACKAGES := ./...
GO_FILES := $(shell find . -type f -name '*.go' -not -path './bin/*')
POSTGRES_DSN ?= postgres://videolibrary:videolibrary@localhost:5432/videolibrary?sslmode=disable
BASE_STACK := docker compose -f ../docker-compose.yml --env-file $(ENV_FILE)

GO_BIN_DIR ?= $(CURDIR)/bin
GOCACHE ?= $(GO_BIN_DIR)/.cache/go-build
GOLANGCI_LINT_CACHE ?= $(GO_BIN_DIR)/.cache/golangci-lint
export GOCACHE
export GOLANGCI_LINT_CACHE

GOOSE_VERSION ?= v3.26.0
GOIMPORTS_VERSION ?= v0.38.0
GOVULNCHECK_VERSION ?= v1.1.4
GOLANGCI_LINT_VERSION ?= v2.11.4
MOCKERY_VERSION ?= v3.5.5
OAPI_CODEGEN_VERSION ?= v2.6.0
REDOCLY_VERSION ?= 1.26.0
SQLC_VERSION ?= v1.31.1
USE_NPX_REDOCLY ?= 0

GOOSE := $(GO_BIN_DIR)/goose
GOIMPORTS := $(GO_BIN_DIR)/goimports
GOLANGCI_LINT := $(GO_BIN_DIR)/golangci-lint
GOVULNCHECK := $(GO_BIN_DIR)/govulncheck
MOCKERY := $(GO_BIN_DIR)/mockery
OAPI_CODEGEN := $(shell if [ -x "$(GO_BIN_DIR)/oapi-codegen" ]; then printf '%s' "$(GO_BIN_DIR)/oapi-codegen"; else command -v oapi-codegen 2>/dev/null || printf '%s' "$(GO_BIN_DIR)/oapi-codegen"; fi)
SQLC := $(GO_BIN_DIR)/sqlc
REDOCLY := npx --yes @redocly/cli@$(REDOCLY_VERSION)
CODEGEN_VENV := .venv-codegen
CODEGEN_PYTHON := $(CODEGEN_VENV)/bin/python
CODEGEN_PIP := $(CODEGEN_VENV)/bin/pip
CODEGEN_REQUIREMENTS := script/requirements-codegen.txt

OPENAPI_ENTRY := internal/openapi/openapi.yml
OPENAPI_BUNDLE := internal/openapi/openapi.bundle.yml
OPENAPI_BUNDLER := script/bundle-openapi.py
OPENAPI_SCHEMA_DIR := internal/openapi/components/schemas
OPENAPI_SCHEMA_FILE := internal/openapi/components/schemas.yml

help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make <target>\n"} /^[a-zA-Z_0-9-]+:.*##/ {printf "  %-24s %s\n", $$1, $$2} /^##@/ {printf "\n%s\n", substr($$0, 5)}' $(MAKEFILE_LIST)

##@ Compose
compose-up: ## Start PostgreSQL, Redis, and SeaweedFS from the root compose file.
	$(BASE_STACK) up -d postgres redis seaweedfs

compose-down: ## Stop the root compose stack.
	$(BASE_STACK) down

compose-logs: ## Tail logs from the root compose stack.
	$(BASE_STACK) logs -f

compose-ps: ## Show root compose service status.
	$(BASE_STACK) ps

##@ Tools
install-tools: install-go-tools ## Install pinned Go tools used by codegen, migrations, linting, and audits.

install-go-tools: ## Install pinned Go tools into GO_BIN_DIR.
	@mkdir -p $(GO_BIN_DIR)
	GOBIN=$(GO_BIN_DIR) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)
	GOBIN=$(GO_BIN_DIR) go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	GOBIN=$(GO_BIN_DIR) go install github.com/vektra/mockery/v3@$(MOCKERY_VERSION)
	GOBIN=$(GO_BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN=$(GO_BIN_DIR) go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
	GOBIN=$(GO_BIN_DIR) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	GOBIN=$(GO_BIN_DIR) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

install-tools-latest: ## Install latest tool versions; intended only for local exploration.
	@mkdir -p $(GO_BIN_DIR)
	GOBIN=$(GO_BIN_DIR) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	GOBIN=$(GO_BIN_DIR) go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	GOBIN=$(GO_BIN_DIR) go install github.com/vektra/mockery/v3@latest
	GOBIN=$(GO_BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	GOBIN=$(GO_BIN_DIR) go install github.com/pressly/goose/v3/cmd/goose@latest
	GOBIN=$(GO_BIN_DIR) go install golang.org/x/tools/cmd/goimports@latest
	GOBIN=$(GO_BIN_DIR) go install golang.org/x/vuln/cmd/govulncheck@latest

install-codegen-python-deps: ## Install Python dependencies for OpenAPI schema merging.
	python3 -m venv $(CODEGEN_VENV)
	$(CODEGEN_PIP) install -r $(CODEGEN_REQUIREMENTS)

##@ Code generation
merge-schemas: ## Merge OpenAPI schema fragments into components/schemas.yml.
	@if [ -x "$(CODEGEN_PYTHON)" ]; then \
		$(CODEGEN_PYTHON) script/merge-schemas.py --source $(OPENAPI_SCHEMA_DIR) --output $(OPENAPI_SCHEMA_FILE); \
	else \
		python3 script/merge-schemas.py --source $(OPENAPI_SCHEMA_DIR) --output $(OPENAPI_SCHEMA_FILE); \
	fi

openapi-bundle: merge-schemas ## Bundle the modular OpenAPI spec.
	@if command -v redocly >/dev/null 2>&1; then \
		redocly bundle $(OPENAPI_ENTRY) --output $(OPENAPI_BUNDLE); \
	elif [ "$(USE_NPX_REDOCLY)" = "1" ]; then \
		$(REDOCLY) bundle $(OPENAPI_ENTRY) --output $(OPENAPI_BUNDLE); \
	else \
		python3 $(OPENAPI_BUNDLER) --entry $(OPENAPI_ENTRY) --output $(OPENAPI_BUNDLE); \
	fi

validate-openapi: openapi-bundle ## Lint the bundled OpenAPI spec.
	@if command -v redocly >/dev/null 2>&1; then \
		redocly lint $(OPENAPI_BUNDLE); \
	elif [ "$(USE_NPX_REDOCLY)" = "1" ]; then \
		$(REDOCLY) lint $(OPENAPI_BUNDLE); \
	else \
		tmp_file="$$(mktemp)"; \
		$(OAPI_CODEGEN) --package openapi --generate types -o "$$tmp_file" $(OPENAPI_BUNDLE); \
		rm -f "$$tmp_file"; \
	fi

oapi-codegen: openapi-bundle ## Generate OpenAPI Go types, chi server, embedded spec, and client.
	$(OAPI_CODEGEN) --config codegen/oapi-codegen-types.yml $(OPENAPI_BUNDLE)
	$(OAPI_CODEGEN) --config codegen/oapi-codegen-server.yml $(OPENAPI_BUNDLE)
	$(OAPI_CODEGEN) --config codegen/oapi-codegen-spec.yml $(OPENAPI_BUNDLE)
	$(OAPI_CODEGEN) --config codegen/oapi-codegen-client.yml $(OPENAPI_BUNDLE)
	rm -f $(OPENAPI_BUNDLE) $(OPENAPI_SCHEMA_FILE)

openapi: validate-openapi oapi-codegen ## Validate the OpenAPI spec and generate Go artifacts.

sqlc: ## Generate SQLC code when SQL query files exist.
	@if find internal -path '*/query.sql' -o -path '*/queries.sql' -o -path '*/queries/*.sql' | grep -q .; then \
		$(SQLC) generate -f codegen/sqlc.yaml; \
	else \
		echo "sqlc: no query files found, skipping"; \
	fi

mockery: ## Generate mocks for configured packages.
	@if find codegen -maxdepth 1 -name '.mockery_*.yml' | grep -q .; then \
		for cfg in codegen/.mockery_*.yml; do $(MOCKERY) --config "$$cfg"; done; \
	else \
		echo "mockery: no codegen/.mockery_*.yml configs found, skipping"; \
	fi

generate: validate-openapi oapi-codegen sqlc mockery ## Run all code generators.

##@ Migrations
migrate-up: ## Apply all pending database migrations.
	$(GOOSE) -dir migrations postgres "$(POSTGRES_DSN)" up

migrate-down: ## Roll back one database migration.
	$(GOOSE) -dir migrations postgres "$(POSTGRES_DSN)" down

migrate-status: ## Show database migration status.
	$(GOOSE) -dir migrations postgres "$(POSTGRES_DSN)" status

migrate-create: ## Create a migration: make migrate-create name=create_videos_table.
	@test -n "$(name)" || (echo "name is required" && exit 1)
	$(GOOSE) -dir migrations create "$(name)" sql

##@ Build
build: ## Build the application binary.
	go build -o $(APP_BIN) ./cmd/app

run: ## Run the application locally.
	go run ./cmd/app

clean: ## Remove generated local build artifacts.
	rm -rf bin coverage.out coverage.html

##@ Tests
test: ## Run all Go tests.
	go test $(GO_PACKAGES)

test-race: ## Run all Go tests with the race detector.
	go test -race $(GO_PACKAGES)

tese-race: test-race ## Backward-compatible alias for the requested race-test target.

test-unit: test ## Run unit tests.

test-integration: ## Run integration tests.
	go test -tags=integration $(GO_PACKAGES)

test-e2e: ## Run end-to-end tests.
	go test -tags=e2e $(GO_PACKAGES)

##@ Quality
fmt: ## Format Go files with goimports when available, otherwise gofmt.
	@if [ -x "$(GOIMPORTS)" ]; then \
		$(GOIMPORTS) -w $(GO_FILES); \
	elif command -v goimports >/dev/null 2>&1; then \
		goimports -w $(GO_FILES); \
	else \
		gofmt -w $(GO_FILES); \
	fi

mod-tidy: ## Tidy Go module dependencies.
	go mod tidy

lint: ## Run golangci-lint when available, otherwise go vet.
	@if [ -x "$(GOLANGCI_LINT)" ]; then \
		$(GOLANGCI_LINT) run; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		go vet $(GO_PACKAGES); \
	fi

lint-fix: fmt mod-tidy lint test ## Format, tidy, lint, and test the backend.

deps-audit: ## Run govulncheck when available.
	@if [ -x "$(GOVULNCHECK)" ]; then \
		$(GOVULNCHECK) $(GO_PACKAGES); \
	elif command -v govulncheck >/dev/null 2>&1; then \
		govulncheck $(GO_PACKAGES); \
	else \
		echo "govulncheck is not installed; run make install-tools"; \
	fi
