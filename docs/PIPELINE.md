# Phase 1 Pipeline

## Overview

The Phase 1 pipeline generates deterministic report artifacts from stored data.
It orchestrates the reporting and decision packages to produce three output files.

## Architecture

```
+------------------+     +------------------+     +------------------+
|  CandidateStore  |     | TradeRecordStore |     | AggregateStore   |
+--------+---------+     +--------+---------+     +--------+---------+
         |                        |                        |
         v                        v                        v
+------------------------------------------------------------------------+
|                        Phase1Pipeline                                   |
|                                                                        |
|  +------------------+    +------------------+    +------------------+  |
|  | reporting.       |    | decision.        |    | decision.        |  |
|  | Generator        |--->| Builder          |--->| Evaluator        |  |
|  +------------------+    +------------------+    +------------------+  |
|                                                                        |
+------------------------------------------------------------------------+
         |                        |                        |
         v                        v                        v
+------------------+     +------------------+     +---------------------+
| REPORT_PHASE1.md |     | STRATEGY_        |     | DECISION_GATE_      |
|                  |     | AGGREGATES.csv   |     | REPORT.md           |
+------------------+     +------------------+     +---------------------+
```

## Determinism

The pipeline guarantees identical outputs for identical inputs:

1. **Fixed Clock**: A fixed timestamp is injected via `WithClock()` method.
   Default: `2025-01-04 12:00:00 UTC`

2. **Stable Ordering**: All collections are sorted before iteration:
   - Strategy metrics: `(strategy_id, scenario_id, entry_event_type)`
   - Decision inputs: `(strategy_id, entry_event_type)`
   - All CSV/Markdown rows maintain deterministic order

3. **No Random State**: No random number generators or system-dependent values.

## Storage Modes

The pipeline supports two storage backends:

### Production Mode (Default)

Uses PostgreSQL for raw data and ClickHouse for analytics. This is the required mode for MVP.

```bash
# Set DSN environment variables or use flags
export POSTGRES_DSN="postgres://user:pass@localhost:5432/solana_lab?sslmode=disable"
export CLICKHOUSE_DSN="clickhouse://localhost:9000/solana_lab"

# Run pipeline
go run cmd/pipeline/main.go --output-dir ./output

# Or with explicit flags
go run cmd/pipeline/main.go \
  --postgres-dsn "postgres://user:pass@localhost:5432/solana_lab?sslmode=disable" \
  --clickhouse-dsn "clickhouse://localhost:9000/solana_lab" \
  --output-dir ./output
```

### Demo Mode (Fixtures)

Uses in-memory stores with fixture data. **For development and demos only.**

```bash
go run cmd/pipeline/main.go --use-fixtures --output-dir ./output
```

> **Warning:** Fixtures mode uses hardcoded demo data with only 3 candidates.
> Production runs MUST use database-backed storage.

## Usage

```bash
# Production: Run with databases (default)
go run cmd/pipeline/main.go \
  --postgres-dsn "$POSTGRES_DSN" \
  --clickhouse-dsn "$CLICKHOUSE_DSN" \
  --output-dir ./output

# Demo: Run with fixtures (development only)
go run cmd/pipeline/main.go --use-fixtures --output-dir ./output
```

## Output Files

### REPORT_PHASE1.md

Full Phase 1 report containing:
- Data summary (candidates, trades, date range)
- Strategy metrics table
- Source comparison (NEW_TOKEN vs ACTIVE_TOKEN)
- Scenario sensitivity analysis
- Replay references

### STRATEGY_AGGREGATES.csv

CSV export of strategy metrics:
```
strategy_id,scenario_id,entry_event_type,total_trades,win_rate,outcome_mean,...
```

### DECISION_GATE_REPORT.md

GO/NO-GO decision for each strategy containing:
- Decision header (GO or NO-GO)
- GO Criteria checklist (5 criteria)
- NO-GO Triggers checklist (4 triggers)
- Summary with reasons for NO-GO

## Testing

The pipeline includes unit tests verifying:

1. **File Generation**: All three files are created
2. **Determinism**: Multiple runs produce identical outputs
3. **Format Validation**: Output structure matches expected format

Run tests:
```bash
go test ./internal/pipeline/... -v
```

## Fixture Data (Demo Only)

> **Warning:** Fixtures are for development and demos only. Do not use for production analysis.

When using `--use-fixtures`, the pipeline uses in-memory stores populated
with fixture data via `LoadFixtures()`. This includes:

- 3 token candidates (2 NEW_TOKEN, 1 ACTIVE_TOKEN)
- 5 trade records across realistic/degraded scenarios
- 5 strategy aggregates with realistic metrics

For production analysis, use database-backed storage with `--postgres-dsn` and `--clickhouse-dsn`.

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `--postgres-dsn` | `$POSTGRES_DSN` | PostgreSQL connection string (required unless `--use-fixtures`) |
| `--clickhouse-dsn` | `$CLICKHOUSE_DSN` | ClickHouse connection string (required unless `--use-fixtures`) |
| `--use-fixtures` | `false` | Use in-memory fixtures instead of databases (demo only) |
| `--output-dir` | `docs` | Directory for generated files |
| `--verbose` | `false` | Verbose output |

## Dependencies

- `internal/reporting` - Report generation and rendering
- `internal/decision` - GO/NO-GO evaluation logic
- `internal/storage/memory` - In-memory store implementations
