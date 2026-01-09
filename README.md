# Solana Token Lab - Phase 1 Research Platform

Deterministic research platform for analyzing high-risk Solana tokens and deciding GO/NO-GO on automated trading. Phase 1 focuses on data collection, deterministic replay, backtests, and metrics - **no live trading**.

## Quick Start

### 1. Prerequisites
- Docker & Docker Compose
- Go 1.22+
- Helius API key (get from https://helius.dev)

### 2. Setup

```bash
# Clone and enter directory
cd solana-token-lab

# Copy environment template and add your API key
cp .env.example .env
# Edit .env and set HELIUS_API_KEY

# Start databases
make up

```

### 3. Run Ingestion (collect data)

```bash
# Option A: Run in Docker
make up-ingest

# Option B: Run locally
make ingest
```

### 4. Generate Reports (after 7+ days of data)

```bash
make pipeline   # Run analysis pipeline
make report     # Generate GO/NO-GO report
```

Reports will be saved to `./output/`.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Solana Mainnet                           │
│                    (Helius RPC + WebSocket)                     │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Ingestion Service                           │
│  • NEW_TOKEN detection (first swap per mint)                    │
│  • ACTIVE_TOKEN detection (volume/swap spikes)                  │
│  • Swap events, Liquidity events, Token metadata                │
└───────────────────────────┬─────────────────────────────────────┘
                            │
              ┌─────────────┴─────────────┐
              ▼                           ▼
┌─────────────────────────┐   ┌─────────────────────────┐
│      PostgreSQL         │   │      ClickHouse         │
│  • token_candidates     │   │  • price_timeseries     │
│  • swaps                │   │  • liquidity_timeseries │
│  • swap_events          │   │  • derived_features     │
│  • liquidity_events     │   │  • strategy_aggregates  │
│  • token_metadata       │   │                         │
│  • trade_records        │   │                         │
└─────────────────────────┘   └─────────────────────────┘
              │                           │
              └─────────────┬─────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Pipeline Service                            │
│  • Normalization (price/liquidity timeseries)                   │
│  • Strategy simulation (TIME_EXIT, TRAILING_STOP, LIQUIDITY)    │
│  • 4 scenarios (Optimistic, Realistic, Pessimistic, Degraded)   │
│  • Metrics aggregation                                          │
└───────────────────────────┬─────────────────────────────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Report Generation                           │
│  • REPORT_PHASE1.md                                             │
│  • DECISION_GATE_REPORT.md (GO / NO-GO / INSUFFICIENT_DATA)     │
│  • CSV exports (strategy_aggregates, trade_records, outcomes)   │
└─────────────────────────────────────────────────────────────────┘
```

---

## Commands

```bash
# Development
make build          # Build all binaries
make test           # Run all tests
make lint           # Run linter

# Docker
make up             # Start PostgreSQL + ClickHouse
make up-ingest      # Start databases + ingestion service
make down           # Stop all containers
make logs           # View container logs
make ps             # Show container status
make clean          # Remove containers and volumes

# Database
make psql           # Connect to PostgreSQL
make clickhouse-cli # Connect to ClickHouse

# Run services locally
make ingest         # Run ingestion
make pipeline       # Run analysis pipeline
make report         # Generate reports
```

---

## Scope (Phase 1)

- Discover NEW_TOKEN and ACTIVE_TOKEN candidates
- Append-only raw on-chain data storage
- Deterministic normalization and feature computation
- Strategy backtest simulation (no real transactions)
- Metrics, reporting, and reproducible GO/NO-GO decision

## Non-Goals

- Live trading or transaction submission
- ML/AI/LLM-driven decision-making
- Production-grade scaling or UI

## Key Principles

- **Falsifiability:** system must prove when strategies do not work
- **Determinism:** same input → same output
- **Replayability:** any report can be reproduced from stored data

---

## Documentation

| Document | Description |
|----------|-------------|
| `docs/BRD.md` | Business requirements |
| `docs/MVP_CRITERIA.md` | Acceptance criteria |
| `docs/DECISION_GATE.md` | GO/NO-GO decision rules |
| `docs/STRATEGY_CATALOG.md` | Available strategies |
| `docs/EXECUTION_SCENARIOS.md` | Scenario parameters |
| `docs/DISCOVERY_SPEC.md` | Discovery logic |
| `docs/NORMALIZATION_SPEC.md` | Time series formulas |
| `docs/SIMULATION_SPEC.md` | Backtest behavior |

---

## Code Layout

```
cmd/
├── ingest/     # Data collection from Solana
├── pipeline/   # Analysis pipeline
├── report/     # Report generation
├── replay/     # Replay discovery from stored events
└── backtest/   # Run backtests

internal/
├── domain/         # Domain models
├── storage/        # Storage interfaces (memory, postgres, clickhouse)
├── ingestion/      # Data ingestion
├── discovery/      # Token discovery (NEW_TOKEN, ACTIVE_TOKEN)
├── normalization/  # Timeseries normalization
├── strategy/       # Exit strategies
├── simulation/     # Trade simulation
├── metrics/        # Metrics computation
├── decision/       # GO/NO-GO evaluation
├── pipeline/       # Phase 1 orchestration
├── reporting/      # Report generation
└── solana/         # RPC/WS clients

sql/
├── postgres/    # PostgreSQL migrations
└── clickhouse/  # ClickHouse migrations
```

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIUS_API_KEY` | Helius API key | required |
| `SOLANA_RPC_ENDPOINT` | Solana RPC URL | constructed from API key |
| `SOLANA_WS_ENDPOINT` | Solana WebSocket URL | constructed from API key |
| `POSTGRES_USER` | PostgreSQL user | `solana` |
| `POSTGRES_PASSWORD` | PostgreSQL password | `solana_secret` |
| `POSTGRES_DB` | PostgreSQL database | `solana_lab` |
| `CLICKHOUSE_USER` | ClickHouse user | `default` |
| `CLICKHOUSE_PASSWORD` | ClickHouse password | (empty) |
| `CLICKHOUSE_DB` | ClickHouse database | `solana_lab` |

---

## Status

**Phase 1: MVP Ready (pending production data)**

- ✅ All code implemented
- ✅ All tests passing
- ⏳ Waiting for 7+ days of production data
- ⏳ Final GO/NO-GO decision pending
