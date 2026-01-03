# Replay Protocol — Phase 1

## Overview

This document defines how to replay any simulated trade deterministically. Replay produces identical results given the same inputs.

---

## 1. Replay Purpose

### 1.1 Use Cases

1. **Verification** — Confirm stored trade_record matches computation
2. **Audit** — Reproduce any historical simulation
3. **Debugging** — Trace exact execution path
4. **Reproducibility** — Defend results to stakeholders

### 1.2 Guarantee

```
GIVEN:
  - Same candidate_id
  - Same strategy_id (with parameters)
  - Same scenario_id (with parameters)
  - Same time series data

THEN:
  - Identical trade_record output
  - Bit-for-bit matching (Float64 precision)
```

---

## 2. Required Inputs

### 2.1 Candidate Data

```
FROM price_timeseries WHERE candidate_id = ?:
  - timestamp_ms
  - price
  - volume
  - slot

FROM liquidity_timeseries WHERE candidate_id = ?:
  - timestamp_ms
  - liquidity
  - liquidity_token
  - liquidity_quote
  - slot

FROM token_candidates WHERE candidate_id = ?:
  - source (NEW_TOKEN / ACTIVE_TOKEN)
  - slot
  - discovered_at
```

### 2.2 Strategy Configuration

```
strategy_config:
  strategy_type:        TEXT    -- "TIME_EXIT" | "TRAILING_STOP" | "LIQUIDITY_GUARD"
  entry_event_type:     TEXT    -- "NEW_TOKEN" | "ACTIVE_TOKEN"

  -- Strategy-specific parameters
  hold_duration_ms:     BIGINT  -- for TIME_EXIT
  trail_pct:            FLOAT64 -- for TRAILING_STOP
  initial_stop_pct:     FLOAT64 -- for TRAILING_STOP
  liquidity_drop_pct:   FLOAT64 -- for LIQUIDITY_GUARD
  max_hold_duration_ms: BIGINT  -- for TRAILING_STOP, LIQUIDITY_GUARD
```

### 2.3 Scenario Configuration

```
scenario_config:
  scenario_id:          TEXT    -- "optimistic" | "realistic" | "pessimistic" | "degraded"
  delay_ms:             BIGINT
  slippage_pct:         FLOAT64
  fee_sol:              FLOAT64
  priority_fee_sol:     FLOAT64
  mev_penalty_pct:      FLOAT64
```

---

## 3. Event Ordering

### 3.1 Canonical Order

**Raw events (PostgreSQL swaps/liquidity_events):**
```
ORDER BY slot ASC, tx_signature ASC, event_index ASC
```

**Time series (ClickHouse price_timeseries/liquidity_timeseries):**
```
ORDER BY timestamp_ms ASC, slot ASC
```

Note: Time series tables do not include `tx_signature` or `event_index`.
Use `timestamp_ms` as primary sort, `slot` as tie-breaker.

### 3.2 Merged Event Stream

For strategies requiring both price and liquidity:

```
merged_events = MERGE(
  price_timeseries,
  liquidity_timeseries
) ORDER BY timestamp_ms ASC, slot ASC

-- If same timestamp_ms, use slot for tie-breaking
-- If same slot, price events before liquidity events (deterministic)
```

---

## 4. Replay Procedure

### 4.1 Initialization

```
FUNCTION replay_trade(candidate_id, strategy_config, scenario_config):

  -- Load data
  price_events = LOAD price_timeseries WHERE candidate_id = ?
                 ORDER BY timestamp_ms ASC

  liquidity_events = LOAD liquidity_timeseries WHERE candidate_id = ?
                     ORDER BY timestamp_ms ASC

  entry_event = LOAD token_candidates WHERE candidate_id = ?

  -- Validate
  ASSERT price_events.length > 0
  ASSERT entry_event.source = strategy_config.entry_event_type
```

### 4.2 Entry Execution

```
  -- Entry signal
  entry_signal_time = entry_event.discovered_at
  entry_signal_price = price_at(entry_signal_time, price_events)  -- price at trigger
  entry_liquidity = liquidity_at(entry_signal_time, liquidity_events)

  -- Apply scenario parameters
  entry_actual_time = entry_signal_time + scenario_config.delay_ms
  entry_actual_price = entry_signal_price * (1 + scenario_config.slippage_pct / 200)
  entry_cost = scenario_config.fee_sol + scenario_config.priority_fee_sol

  -- Position
  position_size = 1.0
  position_value = entry_actual_price * position_size
```

### 4.3 Exit Simulation

```
  -- Initialize state
  peak_price = entry_signal_price
  min_liquidity = entry_liquidity
  exit_signal_time = NULL
  exit_signal_price = NULL
  exit_reason = NULL

  -- Strategy-specific exit loop
  SWITCH strategy_config.strategy_type:

    CASE "TIME_EXIT":
      target_exit_time = entry_signal_time + strategy_config.hold_duration_ms
      exit_signal_time = target_exit_time
      exit_signal_price = price_at(target_exit_time, price_events)
      exit_reason = "TIME_EXIT"

    CASE "TRAILING_STOP":
      initial_stop = entry_signal_price * (1 - strategy_config.initial_stop_pct)

      FOR each price_event IN price_events WHERE timestamp_ms > entry_signal_time:
        t = price_event.timestamp_ms
        price_t = price_event.price

        -- Update peak
        IF price_t > peak_price:
          peak_price = price_t

        -- Calculate trailing stop
        trailing_stop = peak_price * (1 - strategy_config.trail_pct)

        -- Check exits (order matters)
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

        IF (t - entry_signal_time) >= strategy_config.max_hold_duration_ms:
          exit_signal_time = t
          exit_signal_price = price_t
          exit_reason = "MAX_DURATION"
          BREAK

    CASE "LIQUIDITY_GUARD":
      liquidity_threshold = entry_liquidity * (1 - strategy_config.liquidity_drop_pct)

      -- Merge events for this strategy
      FOR each event IN merged_events WHERE timestamp_ms > entry_signal_time:
        t = event.timestamp_ms
        current_liquidity = liquidity_at(t, liquidity_events)
        current_price = price_at(t, price_events)

        -- Track min liquidity
        IF current_liquidity < min_liquidity:
          min_liquidity = current_liquidity

        -- Check exits
        IF current_liquidity < liquidity_threshold:
          exit_signal_time = t
          exit_signal_price = current_price
          exit_reason = "LIQUIDITY_DROP"
          BREAK

        IF (t - entry_signal_time) >= strategy_config.max_hold_duration_ms:
          exit_signal_time = t
          exit_signal_price = current_price
          exit_reason = "MAX_DURATION"
          BREAK
```

### 4.4 Exit Execution

```
  -- Apply scenario parameters
  exit_actual_time = exit_signal_time + scenario_config.delay_ms
  exit_actual_price = exit_signal_price * (1 - scenario_config.slippage_pct / 200)
  exit_cost = scenario_config.fee_sol + scenario_config.priority_fee_sol
```

### 4.5 Outcome Calculation

```
  -- MEV cost
  mev_cost = position_value * (scenario_config.mev_penalty_pct / 100)

  -- Total cost
  total_cost = entry_cost + exit_cost + mev_cost
  total_cost_pct = total_cost / position_value

  -- Returns
  gross_return = (exit_actual_price - entry_actual_price) / entry_actual_price
  outcome = gross_return - total_cost_pct

  -- Classification
  IF outcome > 0:
    outcome_class = "WIN"
  ELSE:
    outcome_class = "LOSS"

  -- Hold duration
  hold_duration_ms = exit_actual_time - entry_actual_time
```

### 4.6 Build Trade Record

```
  trade_record = {
    trade_id: SHA256(candidate_id|strategy_id|scenario_id|entry_signal_time),
    candidate_id: candidate_id,
    strategy_id: strategy_config.to_string(),
    scenario_id: scenario_config.scenario_id,

    entry_signal_time: entry_signal_time,
    entry_signal_price: entry_signal_price,
    entry_actual_time: entry_actual_time,
    entry_actual_price: entry_actual_price,
    entry_liquidity: entry_liquidity,
    position_size: position_size,
    position_value: position_value,

    exit_signal_time: exit_signal_time,
    exit_signal_price: exit_signal_price,
    exit_actual_time: exit_actual_time,
    exit_actual_price: exit_actual_price,
    exit_reason: exit_reason,

    entry_cost_sol: entry_cost,
    exit_cost_sol: exit_cost,
    mev_cost_sol: mev_cost,
    total_cost_sol: total_cost,
    total_cost_pct: total_cost_pct,

    gross_return: gross_return,
    outcome: outcome,
    outcome_class: outcome_class,

    hold_duration_ms: hold_duration_ms,
    peak_price: peak_price,
    min_liquidity: min_liquidity
  }

  RETURN trade_record
```

---

## 5. Helper Functions

### 5.1 price_at

```
FUNCTION price_at(target_time, price_events):
  -- Find closest price at or before target_time
  FOR i = price_events.length - 1 DOWNTO 0:
    IF price_events[i].timestamp_ms <= target_time:
      RETURN price_events[i].price

  -- If no price before target, use first available
  RETURN price_events[0].price
```

### 5.2 liquidity_at

```
FUNCTION liquidity_at(target_time, liquidity_events):
  -- Find closest liquidity at or before target_time
  FOR i = liquidity_events.length - 1 DOWNTO 0:
    IF liquidity_events[i].timestamp_ms <= target_time:
      RETURN liquidity_events[i].liquidity

  -- If no liquidity event before target, return NULL
  RETURN NULL
```

---

## 6. Verification

### 6.1 Replay Verification Query

```sql
-- Compare replayed trade with stored trade
SELECT
  stored.trade_id,
  stored.outcome AS stored_outcome,
  replayed.outcome AS replayed_outcome,
  ABS(stored.outcome - replayed.outcome) < 0.0000001 AS outcome_matches,
  stored.exit_reason = replayed.exit_reason AS reason_matches
FROM trade_records stored
JOIN replay_results replayed
  ON stored.trade_id = replayed.trade_id
WHERE NOT (
  ABS(stored.outcome - replayed.outcome) < 0.0000001
  AND stored.exit_reason = replayed.exit_reason
);

-- Must return 0 rows for valid replay
```

### 6.2 Full Field Verification

```sql
-- All fields must match
SELECT trade_id
FROM trade_records stored
JOIN replay_results replayed USING (trade_id)
WHERE
  stored.entry_signal_time != replayed.entry_signal_time OR
  stored.entry_signal_price != replayed.entry_signal_price OR
  stored.exit_signal_time != replayed.exit_signal_time OR
  stored.exit_signal_price != replayed.exit_signal_price OR
  stored.exit_reason != replayed.exit_reason OR
  ABS(stored.outcome - replayed.outcome) >= 0.0000001;

-- Must return 0 rows
```

---

## 7. Determinism Guarantees

### 7.1 Requirements

| Requirement | Implementation |
|-------------|----------------|
| No randomness | No RNG calls |
| No external APIs | All data from DB |
| Consistent ordering | ORDER BY timestamp_ms ASC, slot ASC (for time series) |
| Explicit parameters | All thresholds in config |
| Consistent precision | Float64 throughout |
| Stable tie-breaking | timestamp_ms primary, slot secondary |

### 7.2 Floating Point Handling

```
Comparison tolerance: 1e-7 (0.0000001)
Precision: Float64 (IEEE 754 double)
Rounding: None (use exact computation)
```

---

## 8. Replay Audit Trail

### 8.1 Required Metadata

For each replay, record:

```
replay_metadata:
  replay_id:            TEXT    -- unique replay identifier
  trade_id:             TEXT    -- trade being replayed
  replay_timestamp:     BIGINT  -- when replay was executed
  data_version:         TEXT    -- hash of input data
  code_version:         TEXT    -- simulator version
  match_result:         BOOLEAN -- did replay match stored?
```

### 8.2 Data Versioning

```
data_version = SHA256(
  price_timeseries_hash ||
  liquidity_timeseries_hash ||
  strategy_config_hash ||
  scenario_config_hash
)
```

---

## References

- `docs/SIMULATION_SPEC.md` — Simulation rules
- `docs/NORMALIZATION_SPEC.md` — Event ordering
- `docs/STRATEGY_CATALOG.md` — Strategy definitions
- `docs/EXECUTION_SCENARIOS.md` — Scenario parameters
