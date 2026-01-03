# Simulation Specification — Phase 1

## Overview

This document defines deterministic backtest and simulation rules for the three required strategies. All rules are formula-based and reproducible.

---

## 1. Strategy Execution Model

### 1.1 Event-Driven Entry

```
1. Detect entry event:
   - NEW_TOKEN: first swap for a token mint
   - ACTIVE_TOKEN: volume/swap spike detection

2. Capture signal values:
   signal_time = event timestamp (ms)
   signal_price = price at event

3. Apply execution delay:
   entry_time_actual = signal_time + delay_ms

4. Apply slippage (price worsens on buy):
   entry_price_actual = signal_price * (1 + slippage_pct / 200)

5. Record entry cost:
   entry_cost = fee_sol + priority_fee_sol

6. Set position:
   position_size = 1.0 (base units, unless specified)
   position_value = entry_price_actual * position_size
```

### 1.2 Rule-Based Exit

```
1. Evaluate exit conditions per strategy rules

2. On exit trigger:
   exit_signal_time = trigger timestamp (ms)
   exit_signal_price = price at trigger

3. Apply execution delay:
   exit_time_actual = exit_signal_time + delay_ms

4. Apply slippage (price worsens on sell):
   exit_price_actual = exit_signal_price * (1 - slippage_pct / 200)

5. Record exit cost:
   exit_cost = fee_sol + priority_fee_sol

6. Apply MEV penalty:
   mev_cost = position_value * (mev_penalty_pct / 100)
```

### 1.3 Cost Calculation

```
Total Cost:
  total_cost = entry_cost + exit_cost + mev_cost
  total_cost_pct = total_cost / position_value

WHERE:
  entry_cost = fee_sol + priority_fee_sol
  exit_cost = fee_sol + priority_fee_sol
  mev_cost = position_value * (mev_penalty_pct / 100)
  position_value = entry_price_actual * position_size
```

### 1.4 Outcome Calculation

```
gross_return = (exit_price_actual - entry_price_actual) / entry_price_actual

outcome = gross_return - total_cost_pct

outcome_classification:
  IF outcome > 0: WIN
  IF outcome <= 0: LOSS
```

---

## 2. Strategy Definitions

### 2.1 Strategy 1: Event + Time Exit

**Description:** Enter on event trigger, exit after fixed time duration.

**Entry:**
```
trigger = NEW_TOKEN or ACTIVE_TOKEN (configurable)
entry_signal_price = trigger_price
entry_signal_time = trigger_time
```

**Exit:**
```
exit_signal_time = entry_signal_time + hold_duration_ms
exit_signal_price = price_at(exit_signal_time)
exit_reason = "TIME_EXIT"
```

**Parameters:**

| Parameter | Type | Values | Description |
|-----------|------|--------|-------------|
| entry_event_type | enum | NEW_TOKEN, ACTIVE_TOKEN | Entry trigger type |
| hold_duration | seconds | 60, 300, 600, 1800 | Hold duration |

**Exit Reason Codes:**
- `TIME_EXIT` — hold duration elapsed

---

### 2.2 Strategy 2: Event + Trailing Stop

**Description:** Enter on event trigger, exit when price drops from peak.

**Entry:**
```
trigger = NEW_TOKEN or ACTIVE_TOKEN (configurable)
entry_signal_price = trigger_price
entry_signal_time = trigger_time
initial_stop = entry_signal_price * (1 - initial_stop_pct)
peak_price = entry_signal_price
```

**Exit Loop:**
```
FOR each price_event after entry_signal_time:
    t = price_event.timestamp_ms
    price_t = price_event.price

    -- Update peak
    IF price_t > peak_price:
        peak_price = price_t

    -- Calculate trailing stop
    trailing_stop = peak_price * (1 - trail_pct)

    -- Check exit conditions (order matters)
    IF price_t <= initial_stop:
        exit_signal_time = t
        exit_signal_price = price_t
        exit_reason = "INITIAL_STOP"
        BREAK

    IF price_t <= trailing_stop:
        exit_signal_time = t
        exit_signal_price = price_t
        exit_reason = "TRAILING_STOP"
        BREAK

    IF (t - entry_signal_time) >= max_hold_duration_ms:
        exit_signal_time = t
        exit_signal_price = price_t
        exit_reason = "MAX_DURATION"
        BREAK
```

**Parameters:**

| Parameter | Type | Values | Description |
|-----------|------|--------|-------------|
| entry_event_type | enum | NEW_TOKEN, ACTIVE_TOKEN | Entry trigger type |
| trail_pct | decimal | 0.05, 0.10, 0.15, 0.20 | Trailing stop % |
| initial_stop_pct | decimal | 0.10 | Initial stop loss % |
| max_hold_duration | seconds | 3600 | Maximum hold time |

**Exit Reason Codes:**
- `INITIAL_STOP` — price fell below initial stop
- `TRAILING_STOP` — price fell below trailing stop
- `MAX_DURATION` — max hold duration elapsed

---

### 2.3 Strategy 3: Event + Liquidity Guard

**Description:** Enter on event trigger, exit when liquidity drops.

**Entry:**
```
trigger = NEW_TOKEN or ACTIVE_TOKEN (configurable)
entry_signal_price = trigger_price
entry_signal_time = trigger_time
entry_liquidity = liquidity_at(entry_signal_time)
liquidity_threshold = entry_liquidity * (1 - liquidity_drop_pct)
```

**Exit Loop:**
```
FOR each event after entry_signal_time:
    t = event.timestamp_ms
    current_liquidity = liquidity_at(t)
    current_price = price_at(t)

    -- Check liquidity drop
    IF current_liquidity < liquidity_threshold:
        exit_signal_time = t
        exit_signal_price = current_price
        exit_reason = "LIQUIDITY_DROP"
        BREAK

    -- Check max duration
    IF (t - entry_signal_time) >= max_hold_duration_ms:
        exit_signal_time = t
        exit_signal_price = current_price
        exit_reason = "MAX_DURATION"
        BREAK
```

**Parameters:**

| Parameter | Type | Values | Description |
|-----------|------|--------|-------------|
| entry_event_type | enum | NEW_TOKEN, ACTIVE_TOKEN | Entry trigger type |
| liquidity_drop_pct | decimal | 0.20, 0.30, 0.50 | Liquidity drop threshold |
| max_hold_duration | seconds | 1800 | Maximum hold time |

**Exit Reason Codes:**
- `LIQUIDITY_DROP` — liquidity fell below threshold
- `MAX_DURATION` — max hold duration elapsed

---

## 3. Execution Scenario Application

### 3.1 Scenario Parameters

From `docs/EXECUTION_SCENARIOS.md`:

| Parameter | Optimistic | Realistic | Pessimistic | Degraded |
|-----------|------------|-----------|-------------|----------|
| delay_ms | 100 | 500 | 2000 | 5000 |
| slippage_pct | 0.5 | 2.0 | 5.0 | 10.0 |
| fee_sol | 0.000005 | 0.00001 | 0.0001 | 0.001 |
| priority_fee_sol | 0 | 0.0001 | 0.001 | 0.01 |
| mev_penalty_pct | 0 | 1.0 | 3.0 | 5.0 |

### 3.2 Application Formulas

**Entry Execution:**
```
entry_time_actual = entry_signal_time + delay_ms
entry_price_actual = entry_signal_price * (1 + slippage_pct / 200)
entry_cost = fee_sol + priority_fee_sol
```

**Exit Execution:**
```
exit_time_actual = exit_signal_time + delay_ms
exit_price_actual = exit_signal_price * (1 - slippage_pct / 200)
exit_cost = fee_sol + priority_fee_sol
```

**MEV Penalty:**
```
mev_cost = position_value * (mev_penalty_pct / 100)

WHERE:
  position_value = entry_price_actual * position_size
  position_size = 1.0 (default)
```

**Total Cost:**
```
total_cost = entry_cost + exit_cost + mev_cost
total_cost_pct = total_cost / position_value
```

---

## 4. Output Schemas

### 4.1 Per-Trade Record

```sql
CREATE TABLE trade_records (
    -- Identifiers
    trade_id              TEXT PRIMARY KEY,   -- SHA256(candidate_id|strategy_id|scenario_id|entry_signal_time)
    candidate_id          TEXT NOT NULL,
    strategy_id           TEXT NOT NULL,
    scenario_id           TEXT NOT NULL,

    -- Entry
    entry_signal_time     BIGINT NOT NULL,    -- ms
    entry_signal_price    FLOAT64 NOT NULL,
    entry_actual_time     BIGINT NOT NULL,    -- ms
    entry_actual_price    FLOAT64 NOT NULL,
    entry_liquidity       FLOAT64,
    position_size         FLOAT64 NOT NULL,   -- default 1.0
    position_value        FLOAT64 NOT NULL,   -- entry_actual_price * position_size

    -- Exit
    exit_signal_time      BIGINT NOT NULL,    -- ms
    exit_signal_price     FLOAT64 NOT NULL,
    exit_actual_time      BIGINT NOT NULL,    -- ms
    exit_actual_price     FLOAT64 NOT NULL,
    exit_reason           TEXT NOT NULL,      -- reason code

    -- Costs
    entry_cost_sol        FLOAT64 NOT NULL,
    exit_cost_sol         FLOAT64 NOT NULL,
    mev_cost_sol          FLOAT64 NOT NULL,
    total_cost_sol        FLOAT64 NOT NULL,
    total_cost_pct        FLOAT64 NOT NULL,

    -- Outcome
    gross_return          FLOAT64 NOT NULL,
    outcome               FLOAT64 NOT NULL,
    outcome_class         TEXT NOT NULL,      -- WIN / LOSS

    -- Metadata
    hold_duration_ms      BIGINT NOT NULL,
    peak_price            FLOAT64,            -- for trailing stop
    min_liquidity         FLOAT64             -- for liquidity guard
);
```

**trade_id Formula:**
```
trade_id = SHA256(
    candidate_id || '|' ||
    strategy_id || '|' ||
    scenario_id || '|' ||
    entry_signal_time::text
)
```

### 4.2 Per-Strategy Aggregate

```sql
CREATE TABLE strategy_aggregates (
    -- Identifiers
    strategy_id           TEXT NOT NULL,
    scenario_id           TEXT NOT NULL,
    entry_event_type      TEXT NOT NULL,      -- NEW_TOKEN / ACTIVE_TOKEN

    -- Counts
    total_trades          INT NOT NULL,
    wins                  INT NOT NULL,
    losses                INT NOT NULL,

    -- Win Rate
    win_rate              FLOAT64 NOT NULL,   -- wins / total_trades

    -- Outcome Distribution
    outcome_mean          FLOAT64 NOT NULL,
    outcome_median        FLOAT64 NOT NULL,
    outcome_p10           FLOAT64 NOT NULL,
    outcome_p25           FLOAT64 NOT NULL,
    outcome_p75           FLOAT64 NOT NULL,
    outcome_p90           FLOAT64 NOT NULL,
    outcome_min           FLOAT64 NOT NULL,
    outcome_max           FLOAT64 NOT NULL,
    outcome_stddev        FLOAT64 NOT NULL,

    -- Drawdown
    max_drawdown          FLOAT64 NOT NULL,
    max_consecutive_losses INT NOT NULL,

    -- Sensitivity (cross-scenario comparison)
    outcome_realistic     FLOAT64,            -- baseline (Realistic scenario)
    outcome_pessimistic   FLOAT64,            -- Pessimistic scenario
    outcome_degraded      FLOAT64,            -- Degraded scenario

    PRIMARY KEY (strategy_id, scenario_id, entry_event_type)
);
```

### 4.3 Aggregate Formulas

```
win_rate = wins / total_trades

outcome_mean = SUM(outcome) / total_trades

outcome_median = PERCENTILE(outcome, 0.50)

outcome_p10 = PERCENTILE(outcome, 0.10)
outcome_p25 = PERCENTILE(outcome, 0.25)
outcome_p75 = PERCENTILE(outcome, 0.75)
outcome_p90 = PERCENTILE(outcome, 0.90)

outcome_stddev = STDDEV(outcome)

max_drawdown = MAX(peak_cumulative - trough_cumulative)
  WHERE:
    cumulative_return[i] = SUM(outcome[1..i])
    peak = running maximum of cumulative_return
    drawdown = peak - cumulative_return
    max_drawdown = MAX(drawdown)

max_consecutive_losses = longest streak of outcome <= 0
```

---

## 5. Exit Reason Code Reference

| Code | Strategy | Description |
|------|----------|-------------|
| TIME_EXIT | Time Exit | Hold duration elapsed |
| INITIAL_STOP | Trailing Stop | Price fell below initial stop |
| TRAILING_STOP | Trailing Stop | Price fell below trailing stop from peak |
| MAX_DURATION | Trailing Stop, Liquidity Guard | Maximum hold duration elapsed |
| LIQUIDITY_DROP | Liquidity Guard | Liquidity fell below threshold |

---

## 6. Determinism Requirements

1. **Same inputs → same outputs**
   - Candidate data
   - Strategy parameters
   - Scenario parameters
   - → Identical trade_record

2. **No randomness**
   - No random number generation
   - No sampling

3. **No external dependencies**
   - All data from PostgreSQL/ClickHouse
   - No RPC calls during simulation

4. **Explicit parameters**
   - All thresholds explicit
   - No magic numbers

5. **Consistent precision**
   - Float64 for all decimal values
   - BIGINT (ms) for all timestamps

---

## References

- `docs/STRATEGY_CATALOG.md` — Strategy definitions
- `docs/EXECUTION_SCENARIOS.md` — Scenario parameters
- `docs/MVP_CRITERIA.md` — Metrics requirements
- `docs/NORMALIZATION_SPEC.md` — Event ordering
- `docs/REPLAY_PROTOCOL.md` — Replay procedure
