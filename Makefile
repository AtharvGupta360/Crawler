.PHONY: help dev build run test clean infra infra-down migrate web-install web-dev web-build

# Default target
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

# ─────────────────────────────────────────────
# Development
# ─────────────────────────────────────────────

dev: ## Run the server in development mode
	APP_ENV=development go run ./cmd/server/

build: ## Build the binary
	go build -o bin/jobcrawl ./cmd/server/

run: build ## Build and run
	./bin/jobcrawl

test: ## Run all tests
	go test ./... -v -race

clean: ## Clean build artifacts
	rm -rf bin/
	go clean -cache

# ─────────────────────────────────────────────
# Infrastructure
# ─────────────────────────────────────────────

infra: ## Start all infrastructure (Postgres, Redis, ES, Kafka)
	docker compose up -d
	@echo "Waiting for services to be healthy..."
	@docker compose ps

infra-down: ## Stop all infrastructure
	docker compose down

infra-reset: ## Reset all infrastructure (WARNING: deletes all data)
	docker compose down -v
	docker compose up -d

infra-logs: ## Tail infrastructure logs
	docker compose logs -f

# ─────────────────────────────────────────────
# Dependencies
# ─────────────────────────────────────────────

deps: ## Download Go dependencies
	go mod tidy
	go mod download

# ─────────────────────────────────────────────
# Kafka Topics
# ─────────────────────────────────────────────

kafka-topics: ## Create Kafka topics
	docker exec jobcrawl-kafka /opt/kafka/bin/kafka-topics.sh --create --topic jobs.raw --partitions 6 --replication-factor 1 --bootstrap-server localhost:9092 --if-not-exists
	docker exec jobcrawl-kafka /opt/kafka/bin/kafka-topics.sh --create --topic jobs.processed --partitions 6 --replication-factor 1 --bootstrap-server localhost:9092 --if-not-exists
	docker exec jobcrawl-kafka /opt/kafka/bin/kafka-topics.sh --create --topic jobs.alerts --partitions 3 --replication-factor 1 --bootstrap-server localhost:9092 --if-not-exists
	@echo "Topics created successfully"

kafka-list: ## List Kafka topics
	docker exec jobcrawl-kafka /opt/kafka/bin/kafka-topics.sh --list --bootstrap-server localhost:9092

# ─────────────────────────────────────────────
# Frontend (React Dashboard)
# ─────────────────────────────────────────────

web-install: ## Install frontend dependencies
	cd web && npm install

web-dev: ## Run frontend dev server (port 5173, proxies to :8080)
	cd web && npm run dev

web-build: ## Build frontend for production (output: web/dist/)
	cd web && npm run build

