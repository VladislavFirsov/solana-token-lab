# Solana Token Lab — Phase 1 Research Platform

Deterministic research platform for analyzing high‑risk Solana tokens and deciding GO/NO‑GO on automated trading. Phase 1 focuses on data collection, deterministic replay, backtests, and metrics — **no live trading**.

## Scope (Phase 1)
- Discover NEW_TOKEN and ACTIVE_TOKEN candidates.
- Append‑only raw on‑chain data storage.
- Deterministic normalization and feature computation.
- Strategy backtest simulation (no real transactions).
- Metrics, reporting, and reproducible GO/NO‑GO decision.

## Non‑Goals
- Live trading or transaction submission.
- ML/AI/LLM‑driven decision‑making.
- Production‑grade scaling or UI.

## Key Principles
- **Falsifiability:** system must prove when strategies do not work.
- **Determinism:** same input → same output.
- **Replayability:** any report can be reproduced from stored data and strategy version.

## MVP Criteria
See `docs/MVP_CRITERIA.md` for acceptance criteria per component.

## Documentation
- `docs/BRD.md` — business requirements
- `docs/DECISION_GATE.md` — GO/NO‑GO decision rules
- `docs/ROADMAP_PHASE1.md` — milestone gates
- `docs/DISCOVERY_SPEC.md` — discovery logic and candidate_id formula
- `docs/NORMALIZATION_SPEC.md` — time series + feature formulas
- `docs/SIMULATION_SPEC.md` — backtest behavior
- `docs/METRICS_SPEC.md` — required metrics
- `docs/REPORTING_SPEC.md` — report requirements
- `docs/SCHEMA_POSTGRES.md` / `docs/SCHEMA_CLICKHOUSE.md` — storage schemas

## Code Layout
- `cmd/` — CLI stubs
- `internal/domain/` — domain models
- `internal/storage/` — storage interfaces + in‑memory implementations
- `internal/ingestion/` — ingestion skeleton
- `internal/normalization/` — deterministic normalization & features
- `internal/replay/` — replay engine skeleton
- `internal/backtest/` — backtest engine skeleton
- `internal/discovery/` — NEW_TOKEN and ACTIVE_TOKEN discovery
- `internal/solana/` — RPC/WS interfaces and stubs
- `sql/` — PostgreSQL/ClickHouse migrations

## Workflow
Project work is coordinated via `workflow/` tasks:
- `taskN/taskN.md` — task description
- `taskN/plan.md` — implementation plan
- `taskN/approve.md` — plan approval
- `taskN/review.md` — review findings
- `taskN/review.approve` — review approval

See `docs/WORKFLOW.md` for the exact process.

## Development
- Go version: see `go.mod`
- All data access is interface‑driven; no direct RPC usage in Phase 1 processing.

## Status
Phase 1 is in progress. See `docs/ROADMAP_PHASE1.md` for milestone status.
