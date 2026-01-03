# Decision Checklist — Phase 1

## Overview

This document provides a deterministic checklist for Phase 1 GO/NO-GO decision making. All criteria reference `docs/DECISION_GATE.md` and `docs/MVP_CRITERIA.md`.

---

## 1. Data Sufficiency

**Source:** DECISION_GATE.md Section 1, MVP_CRITERIA.md Sections 1-4

All items must pass before evaluation proceeds.

| # | Criterion | Threshold | Actual | Pass |
|---|-----------|-----------|--------|------|
| 1 | Unique NEW_TOKEN candidates discovered | ≥300 | ___ | [ ] |
| 2 | Continuous discovery uptime | ≥7 days | ___ | [ ] |
| 3 | Data available for backtest | ≥14 days | ___ | [ ] |
| 4 | Duplicate candidate_id count | 0 | ___ | [ ] |
| 5 | Missing events in evaluation period | 0 | ___ | [ ] |
| 6 | Tokens fully replayable from stored data | 100% | ___ | [ ] |

**Verification queries:**

```sql
-- #1: Unique NEW_TOKEN count
SELECT COUNT(DISTINCT candidate_id)
FROM token_candidates
WHERE source = 'NEW_TOKEN';

-- #2: Discovery uptime (days between first and last)
SELECT (MAX(discovered_at) - MIN(discovered_at)) / 86400000 AS days
FROM token_candidates;

-- #3: Backtest data range
SELECT (MAX(timestamp_ms) - MIN(timestamp_ms)) / 86400000 AS days
FROM price_timeseries;

-- #4: Duplicate check
SELECT COUNT(*)
FROM (
    SELECT candidate_id, COUNT(*) AS cnt
    FROM token_candidates
    GROUP BY candidate_id
    HAVING COUNT(*) > 1
);

-- #5: Missing events (implementation-specific)
-- Verify via replay tool

-- #6: Replay success rate
-- Run: go run cmd/replay/main.go --verify-all
```

**Result:**
- [ ] ALL PASS → proceed to Section 2
- [ ] ANY FAIL → **RESULT: INSUFFICIENT_DATA** (STOP)

---

## 2. GO Criteria

**Source:** DECISION_GATE.md Section 3, MVP_CRITERIA.md Section 8

All must be TRUE under **Realistic scenario** for best-performing strategy.

| # | Criterion | Threshold | Actual | Pass |
|---|-----------|-----------|--------|------|
| 1 | Tokens with positive outcome | ≥5% | ___ | [ ] |
| 2 | Median outcome | >0 | ___ | [ ] |
| 3 | Result stable under parameter degradation | per MVP criteria | ___ | [ ] |
| 4 | Result not driven by outliers | quantiles reported and checked | ___ | [ ] |
| 5 | Entry/exit events implementable in realtime | per MVP criteria | ___ | [ ] |

**Verification:**

```sql
-- #1: Positive outcome rate (Realistic scenario)
SELECT
    strategy_id,
    SUM(CASE WHEN outcome > 0 THEN 1 ELSE 0 END)::FLOAT / COUNT(*) AS positive_rate
FROM trade_records
WHERE scenario_id = 'realistic'
GROUP BY strategy_id;

-- #2: Median outcome (Realistic scenario)
SELECT
    strategy_id,
    MEDIAN(outcome) AS median_outcome
FROM trade_records
WHERE scenario_id = 'realistic'
GROUP BY strategy_id;

-- #3: Outcome across scenarios (for stability check per MVP)
SELECT
    strategy_id,
    scenario_id,
    MEDIAN(outcome) AS median_outcome
FROM trade_records
GROUP BY strategy_id, scenario_id;

-- #4: Quantile distribution (Realistic scenario)
SELECT
    strategy_id,
    QUANTILE(outcome, 0.10) AS p10,
    QUANTILE(outcome, 0.25) AS p25,
    QUANTILE(outcome, 0.50) AS p50,
    QUANTILE(outcome, 0.75) AS p75,
    QUANTILE(outcome, 0.90) AS p90
FROM trade_records
WHERE scenario_id = 'realistic'
GROUP BY strategy_id;
```

**Criterion #3 interpretation:**
- Compare outcome across scenarios (Realistic vs Pessimistic vs Degraded)
- Assess per MVP criteria: "result stable under parameter degradation"

**Criterion #4 interpretation:**
- Report full quantile distribution
- Assess per DECISION_GATE.md: "quantiles reported and checked"

**Criterion #5 verification:**
- Entry signal available from stored events (no RPC)
- Exit conditions calculable from stored data
- Verified via: go run cmd/replay/main.go --check-implementable

**Result:**
- [ ] ALL PASS → candidate for GO (proceed to Section 3)
- [ ] ANY FAIL → **RESULT: NO-GO**

---

## 3. NO-GO Triggers

**Source:** DECISION_GATE.md Section 4, MVP_CRITERIA.md Section 8

ANY trigger results in NO-GO.

| # | Trigger | Condition | Actual | Triggered |
|---|---------|-----------|--------|-----------|
| 1 | Low positive rate | <5% tokens positive | ___ | [ ] |
| 2 | Negative median | median_outcome ≤ 0 | ___ | [ ] |
| 3 | Edge disappears under degradation | per MVP criteria | ___ | [ ] |
| 4 | Entry not implementable | per MVP criteria | ___ | [ ] |

**Verification:**

Same queries as GO Criteria #1-5, but checking for trigger conditions.

**Result:**
- [ ] ANY TRIGGERED → **RESULT: NO-GO**
- [ ] NONE TRIGGERED → proceed to Section 4

---

## 4. Decision Logic

```
PROCEDURE evaluate_phase1():

    -- Step 1: Data Sufficiency
    IF any Data Sufficiency check fails:
        RETURN "INSUFFICIENT_DATA"

    -- Step 2: Find best strategy under Realistic
    best_strategy = strategy with highest median_outcome
                    WHERE scenario = 'realistic'

    -- Step 3: Check NO-GO triggers for best strategy
    IF positive_rate < 0.05:
        RETURN "NO-GO" (reason: low positive rate)

    IF median_outcome <= 0:
        RETURN "NO-GO" (reason: negative median)

    IF edge_disappears_under_degradation:  -- per MVP criteria
        RETURN "NO-GO" (reason: edge disappears)

    IF not implementable:  -- per MVP criteria
        RETURN "NO-GO" (reason: not implementable)

    -- Step 4: Check GO criteria for best strategy
    IF positive_rate >= 0.05
       AND median_outcome > 0
       AND result_stable_under_degradation  -- per MVP criteria
       AND quantiles_reported_and_checked
       AND implementable:
        RETURN "GO"

    -- Step 5: Default
    RETURN "NO-GO" (reason: criteria not met)
```

---

## 5. Decision Record Template

```
============================================
PHASE 1 DECISION RECORD
============================================

Date: ___________________
Evaluator: ___________________
Strategy Version: ___________________
Replay Commit: ___________________
Data Version: ___________________

--------------------------------------------
DATA SUFFICIENCY
--------------------------------------------
| # | Criterion              | Threshold | Actual | Pass |
|---|------------------------|-----------|--------|------|
| 1 | NEW_TOKEN candidates   | ≥300      |        | [ ]  |
| 2 | Discovery uptime       | ≥7 days   |        | [ ]  |
| 3 | Backtest data          | ≥14 days  |        | [ ]  |
| 4 | Duplicate candidate_id | 0         |        | [ ]  |
| 5 | Missing events         | 0         |        | [ ]  |
| 6 | Replay success rate    | 100%      |        | [ ]  |

Data Sufficiency: PASS / FAIL

--------------------------------------------
BEST STRATEGY (Realistic Scenario)
--------------------------------------------
Strategy ID: ___________________
Entry Event Type: ___________________

| Metric          | Value |
|-----------------|-------|
| Total trades    |       |
| Win rate        |       |
| Median outcome  |       |
| Mean outcome    |       |
| p25             |       |
| p75             |       |
| Max drawdown    |       |

--------------------------------------------
GO CRITERIA
--------------------------------------------
| # | Criterion            | Threshold                    | Actual | Pass |
|---|----------------------|------------------------------| -------|------|
| 1 | Positive rate        | ≥5%                          |        | [ ]  |
| 2 | Median outcome       | >0                           |        | [ ]  |
| 3 | Stable under degradation | per MVP criteria         |        | [ ]  |
| 4 | Not outlier-driven   | quantiles reported           |        | [ ]  |
| 5 | Implementable        | per MVP criteria             |        | [ ]  |

GO Criteria: ___ / 5 passed

--------------------------------------------
NO-GO TRIGGERS
--------------------------------------------
| # | Trigger              | Condition            | Triggered |
|---|----------------------|----------------------|-----------|
| 1 | Low positive rate    | <5%                  | [ ]       |
| 2 | Negative median      | ≤0                   | [ ]       |
| 3 | Edge disappears      | per MVP criteria     | [ ]       |
| 4 | Not implementable    | per MVP criteria     | [ ]       |

NO-GO Triggers: ___ / 4 triggered

============================================
DECISION: GO / NO-GO / INSUFFICIENT_DATA
============================================

Reason: ___________________________________

--------------------------------------------
REPLAY COMMAND
--------------------------------------------
git checkout [commit] && go run cmd/report/main.go --data-version [hash]

--------------------------------------------
SUPPORTING ARTIFACTS
--------------------------------------------
- [ ] report.md
- [ ] trade_records.csv
- [ ] strategy_aggregates.csv
- [ ] scenario_outcomes.csv
- [ ] metrics_queries.sql

============================================
```

---

## 6. Thresholds Reference

All thresholds from MVP_CRITERIA.md and DECISION_GATE.md:

| Threshold | Source | Value | Purpose |
|-----------|--------|-------|---------|
| NEW_TOKEN count | MVP 1.1 | ≥300 | Statistical significance |
| Discovery uptime | MVP 1.1 | ≥7 days | Continuous operation |
| Backtest data | MVP 5 | ≥14 days | Strategy evaluation |
| Duplicate IDs | MVP 1.1 | 0 | Data integrity |
| Missing events | MVP 2 | 0 | Data completeness |
| Replay success | MVP 4 | 100% | Reproducibility |
| Positive rate | MVP 8 | ≥5% | Minimum edge |
| Median outcome | MVP 8 | >0 | Positive expectancy |
| Stability under degradation | MVP 8 | per MVP criteria | Robustness |

**Note:** No new thresholds beyond MVP. All stability and implementability checks reference MVP_CRITERIA.md directly.

---

## References

- `docs/DECISION_GATE.md` — GO/NO-GO decision framework
- `docs/MVP_CRITERIA.md` — all threshold definitions
- `docs/METRICS_SPEC.md` — metric formulas
- `docs/REPORTING_SPEC.md` — report structure
- `docs/EXECUTION_SCENARIOS.md` — scenario definitions
