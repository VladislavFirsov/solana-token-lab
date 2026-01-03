# Metrics Specification — Phase 1

## Overview

This document defines deterministic formulas for all required metrics used in Phase 1 GO/NO-GO decision making.

---

## 1. Core Metrics

### 1.1 Win Rate

```
win_rate = wins / total_trades

WHERE:
  wins = COUNT(trades WHERE outcome > 0)
  total_trades = COUNT(trades)

Result: decimal [0.0, 1.0]
Display: percentage (e.g., 0.05 → "5%")
```

### 1.2 Mean Outcome

```
mean_outcome = SUM(outcome) / total_trades

WHERE:
  outcome = per-trade outcome from trade_records
  total_trades = COUNT(trades)

Result: decimal (can be negative)
```

### 1.3 Median Outcome

```
median_outcome = PERCENTILE(outcome, 0.50)

WHERE:
  -- Sort all outcomes ascending
  -- If count is odd: middle value
  -- If count is even: average of two middle values

Result: decimal (can be negative)
```

### 1.4 Quantile Distribution

```
p10 = PERCENTILE(outcome, 0.10)
p25 = PERCENTILE(outcome, 0.25)
p50 = PERCENTILE(outcome, 0.50)  -- same as median
p75 = PERCENTILE(outcome, 0.75)
p90 = PERCENTILE(outcome, 0.90)

PERCENTILE(values, p):
  n = COUNT(values)
  k = (n - 1) * p
  f = FLOOR(k)
  c = CEIL(k)
  IF f == c:
    RETURN sorted_values[f]
  ELSE:
    RETURN sorted_values[f] * (c - k) + sorted_values[c] * (k - f)

Result: decimal array
```

### 1.5 Standard Deviation

```
stddev = SQRT(SUM((outcome - mean)^2) / (total_trades - 1))

WHERE:
  mean = mean_outcome
  total_trades = COUNT(trades)

Note: Uses sample standard deviation (n-1 denominator)
Result: decimal >= 0
Edge case: if total_trades <= 1, stddev = 0
```

### 1.6 Outcome Extremes

```
outcome_min = MIN(outcome)
outcome_max = MAX(outcome)

Result: decimal
```

---

## 2. Risk Metrics

### 2.1 Maximum Drawdown

```
max_drawdown = MAX(drawdown)

WHERE:
  -- Compute cumulative returns (trades ordered by entry_signal_time ASC)
  cumulative[i] = SUM(outcome[1..i])

  -- Track running peak
  peak[i] = MAX(cumulative[1..i])

  -- Calculate drawdown at each point
  drawdown[i] = peak[i] - cumulative[i]

Result: decimal >= 0
Interpretation: maximum drop from peak cumulative return
```

**Detailed Algorithm:**

```
INPUT: outcomes[] -- array of trade outcomes in time order

IF LEN(outcomes) == 0:
    RETURN 0

cumulative = 0
peak = 0
max_dd = 0

FOR i = 0 TO LEN(outcomes) - 1:
    cumulative = cumulative + outcomes[i]
    IF cumulative > peak:
        peak = cumulative
    drawdown = peak - cumulative
    IF drawdown > max_dd:
        max_dd = drawdown

RETURN max_dd
```

### 2.2 Maximum Consecutive Losses

```
max_consecutive_losses = longest streak of consecutive trades with outcome <= 0

Algorithm:
  current_streak = 0
  max_streak = 0
  FOR each trade in time order:
    IF outcome <= 0:
      current_streak = current_streak + 1
      max_streak = MAX(max_streak, current_streak)
    ELSE:
      current_streak = 0
  RETURN max_streak

Result: integer >= 0
```

---

## 3. Comparison Rules

### 3.1 NEW_TOKEN vs ACTIVE_TOKEN Comparison

Compare same strategy, same scenario, different entry_event_type:

```
delta_win_rate = win_rate_NEW_TOKEN - win_rate_ACTIVE_TOKEN
delta_median = median_NEW_TOKEN - median_ACTIVE_TOKEN
delta_mean = mean_NEW_TOKEN - mean_ACTIVE_TOKEN

Report both:
  - Absolute values for each source
  - Delta values (NEW_TOKEN - ACTIVE_TOKEN)

Interpretation:
  - Positive delta: NEW_TOKEN outperforms ACTIVE_TOKEN
  - Negative delta: ACTIVE_TOKEN outperforms NEW_TOKEN
```

### 3.2 Strategy vs Strategy Comparison

Compare same entry_event_type, same scenario, different strategy:

```
Ranking criteria (in order):
  1. median_outcome DESC
  2. win_rate DESC
  3. max_drawdown ASC (lower is better)

Report for each strategy:
  - Full quantile distribution (p10, p25, p50, p75, p90)
  - win_rate
  - max_drawdown
  - max_consecutive_losses
```

### 3.3 Scenario Comparison Matrix

For each strategy, report metrics across all scenarios:

```
| Metric        | Optimistic | Realistic | Pessimistic | Degraded |
|---------------|------------|-----------|-------------|----------|
| win_rate      | ___        | ___       | ___         | ___      |
| median        | ___        | ___       | ___         | ___      |
| mean          | ___        | ___       | ___         | ___      |
| p10           | ___        | ___       | ___         | ___      |
| p90           | ___        | ___       | ___         | ___      |
| max_drawdown  | ___        | ___       | ___         | ___      |
```

---

## 4. Aggregation Rules

### 4.1 Grouping Hierarchy

```
Level 1: strategy_id + scenario_id + entry_event_type
  -- Primary grouping for all metrics

Level 2: strategy_id + scenario_id
  -- Aggregated across entry_event_types

Level 3: strategy_id
  -- Aggregated across all scenarios (for overview only)
```

### 4.2 Trade Ordering

```
ORDER BY entry_signal_time ASC

Purpose:
  - Consistent cumulative calculations
  - Deterministic max_drawdown
  - Deterministic max_consecutive_losses
```

### 4.3 NULL Handling

```
Metrics with NULL outcomes:
  - Exclude from aggregation
  - Report count of excluded trades separately

Missing scenarios:
  - Comparison deltas = NULL
```

---

## 5. Metric Output Schema

### 5.1 Per-Group Metrics Record

```sql
-- Reference: strategy_aggregates table from SIMULATION_SPEC.md

strategy_id           TEXT NOT NULL
scenario_id           TEXT NOT NULL
entry_event_type      TEXT NOT NULL      -- NEW_TOKEN / ACTIVE_TOKEN

-- Counts
total_trades          INT NOT NULL
wins                  INT NOT NULL
losses                INT NOT NULL

-- Core metrics
win_rate              FLOAT64 NOT NULL
outcome_mean          FLOAT64 NOT NULL
outcome_median        FLOAT64 NOT NULL
outcome_stddev        FLOAT64 NOT NULL
outcome_min           FLOAT64 NOT NULL
outcome_max           FLOAT64 NOT NULL

-- Quantiles
outcome_p10           FLOAT64 NOT NULL
outcome_p25           FLOAT64 NOT NULL
outcome_p75           FLOAT64 NOT NULL
outcome_p90           FLOAT64 NOT NULL

-- Risk metrics
max_drawdown          FLOAT64 NOT NULL
max_consecutive_losses INT NOT NULL
```

### 5.2 Cross-Scenario Outcomes Record

```sql
strategy_id           TEXT NOT NULL
entry_event_type      TEXT NOT NULL

-- Outcome per scenario (median_outcome)
outcome_optimistic    FLOAT64
outcome_realistic     FLOAT64
outcome_pessimistic   FLOAT64
outcome_degraded      FLOAT64
```

---

## 6. Precision Requirements

```
All decimal values:
  - Storage: FLOAT64
  - Display: 4 decimal places for ratios, 6 for costs

All counts:
  - Storage: INT or BIGINT
  - Display: integer

Percentages:
  - Storage: decimal [0.0, 1.0]
  - Display: "X.XX%" format
```

---

## References

- `docs/SIMULATION_SPEC.md` — trade_records, strategy_aggregates schemas
- `docs/EXECUTION_SCENARIOS.md` — scenario definitions
- `docs/DECISION_GATE.md` — GO/NO-GO criteria
- `docs/MVP_CRITERIA.md` — required metrics list (Section 6)
