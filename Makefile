.PHONY: help build test lint run-local up down logs ps clean migrate

# Default target
help:
	@echo "Solana Token Lab - Available commands:"
	@echo ""
	@echo "  Development:"
	@echo "    make build          - Build all binaries locally"
	@echo "    make test           - Run all tests"
	@echo "    make lint           - Run linter"
	@echo ""
	@echo "  Docker:"
	@echo "    make up             - Start PostgreSQL + ClickHouse"
	@echo "    make up-ingest      - Start databases + ingestion service"
	@echo "    make down           - Stop all containers"
	@echo "    make logs           - View container logs"
	@echo "    make ps             - Show container status"
	@echo "    make clean          - Remove containers and volumes"
	@echo ""
	@echo "  Database:"
	@echo "    make migrate        - Apply SQL migrations"
	@echo "    make psql           - Connect to PostgreSQL"
	@echo "    make clickhouse-cli - Connect to ClickHouse"
	@echo ""
	@echo "  Run services:"
	@echo "    make ingest         - Run ingestion locally"
	@echo "    make pipeline       - Run pipeline locally"
	@echo "    make report         - Generate reports locally"

# =============================================================================
# Development
# =============================================================================

build:
	@echo "Building binaries..."
	go build -o bin/ingest ./cmd/ingest
	go build -o bin/pipeline ./cmd/pipeline
	go build -o bin/report ./cmd/report
	go build -o bin/replay ./cmd/replay
	go build -o bin/backtest ./cmd/backtest
	@echo "Done. Binaries in ./bin/"

test:
	@echo "Running tests..."
	go test ./... -v

test-short:
	@echo "Running short tests (skip integration)..."
	go test ./... -short -v

lint:
	@echo "Running linter..."
	golangci-lint run ./...

# =============================================================================
# Docker
# =============================================================================

up:
	@echo "Starting PostgreSQL and ClickHouse..."
	docker-compose up -d postgres clickhouse
	@echo "Waiting for databases to be ready..."
	@sleep 5
	@echo "Databases ready!"
	@echo "  PostgreSQL: localhost:5432"
	@echo "  ClickHouse: localhost:8123 (HTTP), localhost:9000 (Native)"

up-dev:
	@echo "Starting databases + Adminer..."
	docker-compose --profile dev up -d

up-ingest:
	@echo "Starting databases + ingestion service..."
	docker-compose --profile ingest up -d

down:
	docker-compose --profile ingest --profile pipeline --profile report --profile dev down

logs:
	docker-compose logs -f

logs-ingest:
	docker-compose logs -f ingest

ps:
	docker-compose ps -a

clean:
	@echo "Stopping and removing containers, volumes..."
	docker-compose --profile ingest --profile pipeline --profile report --profile dev down -v
	@echo "Done."

# =============================================================================
# Database
# =============================================================================

migrate:
	@echo "Applying PostgreSQL migrations..."
	@for f in sql/postgres/*.sql; do \
		echo "Applying $$f..."; \
		PGPASSWORD=$${POSTGRES_PASSWORD:-solana_secret} psql -h localhost -U $${POSTGRES_USER:-solana} -d $${POSTGRES_DB:-solana_lab} -f $$f; \
	done
	@echo "Applying ClickHouse migrations..."
	@for f in sql/clickhouse/*.sql; do \
		echo "Applying $$f..."; \
		clickhouse-client --host localhost --query "$$(cat $$f)"; \
	done
	@echo "Migrations complete."

psql:
	PGPASSWORD=$${POSTGRES_PASSWORD:-solana_secret} psql -h localhost -U $${POSTGRES_USER:-solana} -d $${POSTGRES_DB:-solana_lab}

clickhouse-cli:
	clickhouse-client --host localhost

# =============================================================================
# Run Services Locally
# =============================================================================

ingest: build
	@echo "Starting ingestion..."
	@if [ -f .env ]; then export $$(cat .env | grep -v '^#' | xargs); fi && \
	./bin/ingest \
		--rpc-endpoint "https://mainnet.helius-rpc.com/?api-key=$${HELIUS_API_KEY}" \
		--ws-endpoint "wss://mainnet.helius-rpc.com/?api-key=$${HELIUS_API_KEY}" \
		--dex raydium,pumpfun \
		--postgres-dsn "postgres://$${POSTGRES_USER:-solana}:$${POSTGRES_PASSWORD:-solana_secret}@localhost:5432/$${POSTGRES_DB:-solana_lab}?sslmode=disable" \
		--clickhouse-dsn "clickhouse://localhost:9000/$${CLICKHOUSE_DB:-solana_lab}"

pipeline: build
	@echo "Running pipeline..."
	@if [ -f .env ]; then export $$(cat .env | grep -v '^#' | xargs); fi && \
	./bin/pipeline \
		--postgres-dsn "postgres://$${POSTGRES_USER:-solana}:$${POSTGRES_PASSWORD:-solana_secret}@localhost:5432/$${POSTGRES_DB:-solana_lab}?sslmode=disable" \
		--clickhouse-dsn "clickhouse://localhost:9000/$${CLICKHOUSE_DB:-solana_lab}" \
		--output-dir ./output

report: build
	@echo "Generating reports..."
	@if [ -f .env ]; then export $$(cat .env | grep -v '^#' | xargs); fi && \
	./bin/report \
		--postgres-dsn "postgres://$${POSTGRES_USER:-solana}:$${POSTGRES_PASSWORD:-solana_secret}@localhost:5432/$${POSTGRES_DB:-solana_lab}?sslmode=disable" \
		--clickhouse-dsn "clickhouse://localhost:9000/$${CLICKHOUSE_DB:-solana_lab}" \
		--output-dir ./output
