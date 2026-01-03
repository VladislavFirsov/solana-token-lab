# Reporting Specification — Phase 1

## Overview

This document defines the structure, format, and reproducibility requirements for Phase 1 reports.

---

## 1. Report Structure

### 1.1 Executive Summary

```
Section: Executive Summary
Location: Top of report

Contents:
  1. Decision: GO / NO-GO / INSUFFICIENT_DATA
  2. Key metrics summary (one line each):
     - Best strategy: [strategy_id]
     - Win rate (Realistic): [X.XX%]
     - Median outcome (Realistic): [±X.XXXX]
     - Median outcome (Pessimistic): [±X.XXXX]
  3. Data period: [start_date] to [end_date]
  4. Token count: [N] NEW_TOKEN, [M] ACTIVE_TOKEN
```

### 1.2 Data Summary

```
Section: Data Summary

Contents:
  1. Token Counts
     | Source       | Count |
     |--------------|-------|
     | NEW_TOKEN    | ___   |
     | ACTIVE_TOKEN | ___   |
     | Total        | ___   |

  2. Time Range
     - Data start: [ISO timestamp]
     - Data end: [ISO timestamp]
     - Duration: [N days]

  3. Data Quality
     | Check              | Required | Actual | Status |
     |--------------------|----------|--------|--------|
     | Duplicate IDs      | 0        | ___    | ___    |
     | Missing events     | 0        | ___    | ___    |
     | Replay success     | 100%     | ___    | ___    |

  4. Discovery Uptime
     - Continuous discovery: [X days]
     - Gaps detected: [N] (list if any)
```

### 1.3 Metrics Tables

```
Section: Metrics Tables

For each (strategy_id, entry_event_type) combination:

Table: [strategy_id] — [entry_event_type]
| Metric                   | Optimistic | Realistic | Pessimistic | Degraded |
|--------------------------|------------|-----------|-------------|----------|
| total_trades             | ___        | ___       | ___         | ___      |
| wins                     | ___        | ___       | ___         | ___      |
| win_rate                 | ___        | ___       | ___         | ___      |
| outcome_mean             | ___        | ___       | ___         | ___      |
| outcome_median           | ___        | ___       | ___         | ___      |
| outcome_p10              | ___        | ___       | ___         | ___      |
| outcome_p25              | ___        | ___       | ___         | ___      |
| outcome_p75              | ___        | ___       | ___         | ___      |
| outcome_p90              | ___        | ___       | ___         | ___      |
| outcome_min              | ___        | ___       | ___         | ___      |
| outcome_max              | ___        | ___       | ___         | ___      |
| outcome_stddev           | ___        | ___       | ___         | ___      |
| max_drawdown             | ___        | ___       | ___         | ___      |
| max_consecutive_losses   | ___        | ___       | ___         | ___      |
```

### 1.4 Cross-Scenario Outcomes

```
Section: Cross-Scenario Outcomes

Table: Outcome by Scenario (median_outcome)
| Strategy        | Entry Type   | Optimistic | Realistic | Pessimistic | Degraded |
|-----------------|--------------|------------|-----------|-------------|----------|
| [strategy_id]   | NEW_TOKEN    | ___        | ___       | ___         | ___      |
| [strategy_id]   | ACTIVE_TOKEN | ___        | ___       | ___         | ___      |
| ...             | ...          | ...        | ...       | ...         | ...      |

Columns:
  - All columns are median_outcome values for the respective scenario
  - No derived ratios or classifications
```

### 1.5 Comparisons

```
Section: Comparisons

5.1 NEW_TOKEN vs ACTIVE_TOKEN (Realistic scenario)
| Strategy        | NEW_TOKEN Win Rate | ACTIVE_TOKEN Win Rate | Delta Win Rate |
|-----------------|--------------------|-----------------------|----------------|
| [strategy_id]   | ___                | ___                   | ___            |
| ...             | ...                | ...                   | ...            |

| Strategy        | NEW_TOKEN Median | ACTIVE_TOKEN Median | Delta Median |
|-----------------|------------------|---------------------|--------------|
| [strategy_id]   | ___              | ___                 | ___          |
| ...             | ...              | ...                 | ...          |

5.2 Strategy Ranking (Realistic scenario, NEW_TOKEN)
| Rank | Strategy        | Median   | Win Rate | Max Drawdown |
|------|-----------------|----------|----------|--------------|
| 1    | [strategy_id]   | ___      | ___      | ___          |
| 2    | [strategy_id]   | ___      | ___      | ___          |
| ...  | ...             | ...      | ...      | ...          |

5.3 Strategy Ranking (Realistic scenario, ACTIVE_TOKEN)
| Rank | Strategy        | Median   | Win Rate | Max Drawdown |
|------|-----------------|----------|----------|--------------|
| 1    | [strategy_id]   | ___      | ___      | ___          |
| ...  | ...             | ...      | ...      | ...          |
```

### 1.6 Reproducibility

```
Section: Reproducibility

Metadata:
  - Report timestamp: [ISO timestamp]
  - Report generator version: [version]
  - Data version (SHA256): [hash]
  - Strategy version: [git commit or semver]
  - Replay commit hash: [git commit]

Replay Command:
  git checkout [commit] && go run cmd/report/main.go --data-version [hash]
```

### 1.7 Decision Checklist

```
Section: Decision Checklist

Reference: docs/DECISION_CHECKLIST.md

[Include filled DECISION_CHECKLIST or reference]
```

---

## 2. Required Artifacts

### 2.1 CSV Exports

**trade_records.csv**
```
Columns:
  trade_id
  candidate_id
  strategy_id
  scenario_id
  entry_signal_time
  entry_signal_price
  entry_actual_time
  entry_actual_price
  entry_liquidity
  position_size
  position_value
  exit_signal_time
  exit_signal_price
  exit_actual_time
  exit_actual_price
  exit_reason
  entry_cost_sol
  exit_cost_sol
  mev_cost_sol
  total_cost_sol
  total_cost_pct
  gross_return
  outcome
  outcome_class
  hold_duration_ms
  peak_price
  min_liquidity

Format:
  - Encoding: UTF-8
  - Delimiter: comma
  - Quote: double-quote for strings
  - Header: first row
  - Decimal: period (.)
  - NULL: empty string
```

**strategy_aggregates.csv**
```
Columns:
  strategy_id
  scenario_id
  entry_event_type
  total_trades
  wins
  losses
  win_rate
  outcome_mean
  outcome_median
  outcome_p10
  outcome_p25
  outcome_p75
  outcome_p90
  outcome_min
  outcome_max
  outcome_stddev
  max_drawdown
  max_consecutive_losses

Format: same as trade_records.csv
```

**scenario_outcomes.csv**
```
Columns:
  strategy_id
  entry_event_type
  outcome_optimistic
  outcome_realistic
  outcome_pessimistic
  outcome_degraded

Format: same as trade_records.csv
```

### 2.2 SQL Exports

**metrics_queries.sql**

Contains reproducible SQL queries to regenerate all metrics from stored data:

```sql
-- File: metrics_queries.sql
-- Purpose: Reproduce Phase 1 metrics from database
-- Data version: [SHA256 hash]
-- Generated: [ISO timestamp]

-- ===========================================
-- 1. Win Rate per Strategy/Scenario
-- ===========================================
SELECT
    strategy_id,
    scenario_id,
    entry_event_type,
    COUNT(*) AS total_trades,
    SUM(CASE WHEN outcome > 0 THEN 1 ELSE 0 END) AS wins,
    SUM(CASE WHEN outcome > 0 THEN 1 ELSE 0 END)::FLOAT / COUNT(*) AS win_rate
FROM trade_records
GROUP BY strategy_id, scenario_id, entry_event_type;

-- ===========================================
-- 2. Outcome Distribution
-- ===========================================
SELECT
    strategy_id,
    scenario_id,
    entry_event_type,
    AVG(outcome) AS outcome_mean,
    MEDIAN(outcome) AS outcome_median,
    QUANTILE(outcome, 0.10) AS outcome_p10,
    QUANTILE(outcome, 0.25) AS outcome_p25,
    QUANTILE(outcome, 0.75) AS outcome_p75,
    QUANTILE(outcome, 0.90) AS outcome_p90,
    MIN(outcome) AS outcome_min,
    MAX(outcome) AS outcome_max,
    STDDEV_SAMP(outcome) AS outcome_stddev
FROM trade_records
GROUP BY strategy_id, scenario_id, entry_event_type;

-- ===========================================
-- 3. Data Quality Checks
-- ===========================================

-- Duplicate candidate_id check
SELECT candidate_id, COUNT(*) AS cnt
FROM token_candidates
GROUP BY candidate_id
HAVING COUNT(*) > 1;

-- Token count by source
SELECT source, COUNT(DISTINCT candidate_id) AS token_count
FROM token_candidates
GROUP BY source;

-- Time range
SELECT
    MIN(discovered_at) AS data_start,
    MAX(discovered_at) AS data_end
FROM token_candidates;
```

---

## 3. Reproducibility Requirements

### 3.1 Required Metadata

```
data_version:
  - SHA256 hash of concatenated input data
  - Input: price_timeseries || liquidity_timeseries || token_candidates
  - Purpose: verify data integrity

strategy_version:
  - Git commit hash OR semantic version
  - Must match code used for simulation

report_timestamp:
  - ISO 8601 format
  - UTC timezone
  - When report was generated

report_generator_version:
  - Version of reporting tool
  - For tracking changes in report format
```

### 3.2 Data Version Computation

```
data_version = SHA256(
    SHA256(price_timeseries_export) ||
    SHA256(liquidity_timeseries_export) ||
    SHA256(token_candidates_export)
)

WHERE exports are:
  - Sorted by primary key
  - Consistent column order
  - NULL represented as empty string
  - Timestamps as Unix ms
```

### 3.3 Replay Command

```
git checkout [replay_commit_hash] && \
  go run cmd/report/main.go \
    --data-version [data_version_hash] \
    --output-dir ./reports/[timestamp]

Expected behavior:
  - Fail if data version mismatch
  - Produce identical report if data matches
  - Exit with error code on any inconsistency
```

### 3.4 Verification Checklist

```
| Check | Command | Expected |
|-------|---------|----------|
| Data version matches | go run cmd/verify/main.go --check-data | PASS |
| Strategy version exists | git show [commit] | Found |
| Report regenerates | go run cmd/report/main.go | Identical output |
| CSV checksums match | sha256sum *.csv | Match stored checksums |
```

---

## 4. Report File Structure

```
reports/
└── [timestamp]/
    ├── report.md                 -- Human-readable report
    ├── report.json               -- Machine-readable report
    ├── trade_records.csv         -- All simulated trades
    ├── strategy_aggregates.csv   -- Per-strategy metrics
    ├── scenario_outcomes.csv     -- Cross-scenario outcomes
    ├── metrics_queries.sql       -- Reproducible SQL queries
    ├── checksums.sha256          -- File integrity checksums
    └── metadata.json             -- Version metadata
```

### 4.1 metadata.json Schema

```json
{
  "report_timestamp": "2024-01-15T10:30:00Z",
  "report_generator_version": "1.0.0",
  "data_version": "sha256:abc123...",
  "strategy_version": "v1.2.3",
  "replay_commit_hash": "def456...",
  "data_period": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-14T23:59:59Z"
  },
  "token_counts": {
    "NEW_TOKEN": 350,
    "ACTIVE_TOKEN": 120
  },
  "decision": "GO"
}
```

### 4.2 checksums.sha256 Format

```
sha256_hash  report.md
sha256_hash  trade_records.csv
sha256_hash  strategy_aggregates.csv
sha256_hash  scenario_outcomes.csv
sha256_hash  metrics_queries.sql
sha256_hash  metadata.json
```

---

## References

- `docs/METRICS_SPEC.md` — metric formulas
- `docs/DECISION_CHECKLIST.md` — GO/NO-GO checklist
- `docs/DECISION_GATE.md` — decision criteria
- `docs/SIMULATION_SPEC.md` — trade_records schema
- `docs/MVP_CRITERIA.md` — reporting requirements (Section 7)
