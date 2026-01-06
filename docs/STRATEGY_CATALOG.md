# Strategy Catalog — Phase 1

## Purpose
Define the three required trading strategies for Phase 1 simulation with deterministic, reproducible rules.

---

## Constraints

### Required
- All rules expressed as mathematical formulas
- All parameters explicitly defined with ranges
- Deterministic execution (same input → same output)

### Prohibited
- Machine learning or AI-based signals
- Heuristics without explicit formulas
- Subjective interpretation ("looks like", "seems to be")
- Estimated or approximate values
- Manual parameter tuning after seeing results
- Cherry-picking tokens for analysis

---

## Entry Event Definitions

### NEW_TOKEN Entry

**Trigger Condition:**
```
trigger = first_swap_tx WHERE:
    is_first_swap_after_pool_creation = true
    AND pool_exists = true
    AND swap_amount > 0
```

Definition: First swap transaction observed after mint and pool creation events for a token.

**Entry Values:**
```
entry_price = swap_price (from first swap transaction)
entry_time = swap_timestamp
entry_liquidity = pool_liquidity_at(entry_time)
```

**Detection:**
- Monitor pump.fun and similar programs for new token mints
- Detect pool creation events
- Capture first swap transaction

---

### ACTIVE_TOKEN Entry

**Trigger Condition:**
```
trigger = (volume_1h > K_vol * volume_24h_avg)
       OR (swaps_1h > K_swaps * swaps_24h_avg)

WHERE:
    K_vol = 3.0 (configurable)
    K_swaps = 5.0 (configurable)
    volume_24h_avg = total_volume_24h / 24
    swaps_24h_avg = total_swaps_24h / 24
```

**Entry Values:**
```
entry_price = price_at(trigger_detection_time)
entry_time = trigger_detection_time
entry_liquidity = pool_liquidity_at(entry_time)
```

**Detection:**
- Continuous monitoring of existing tokens
- Rolling 1h and 24h windows
- Trigger on threshold crossing

---

## Strategy Definitions

### Strategy 1: Event + Time Exit

**Description:**
Enter on event trigger, exit after fixed time duration.

**Entry:**
```
entry_trigger = NEW_TOKEN or ACTIVE_TOKEN (configurable)
entry_price = trigger_price
entry_time = trigger_time
```

**Exit:**
```
exit_time = entry_time + hold_duration
exit_price = price_at(exit_time)
```

**Parameters:**
| Parameter | Type | Default Range | Description |
|-----------|------|---------------|-------------|
| entry_event_type | enum | {NEW_TOKEN, ACTIVE_TOKEN} | Which trigger to use |
| hold_duration | seconds | {60, 300, 600, 1800} | Time to hold position |

**Outcome Formula:**
```
gross_return = (exit_price - entry_price) / entry_price
outcome = gross_return - total_fee_pct

WHERE:
    total_fee_pct = (entry_fee + exit_fee + mev_penalty) / position_value
```

---

### Strategy 2: Event + Trailing Stop

**Description:**
Enter on event trigger, exit when price drops from peak by trail percentage.

**Entry:**
```
entry_trigger = NEW_TOKEN or ACTIVE_TOKEN (configurable)
entry_price = trigger_price
entry_time = trigger_time
initial_stop = entry_price * (1 - initial_stop_pct)
```

**Exit:**
```
FOR each price_t after entry_time:
    peak_price = MAX(price_t) for all t in [entry_time, current_time]
    trailing_stop = peak_price * (1 - trail_pct)

    -- Check exit conditions (order matters per SIMULATION_SPEC.md)
    IF price_t < initial_stop:
        exit_time = t
        exit_price = price_t
        exit_reason = "INITIAL_STOP"
        BREAK

    IF price_t < trailing_stop:
        exit_time = t
        exit_price = price_t
        exit_reason = "TRAILING_STOP"
        BREAK

    IF (t - entry_time) >= max_hold_duration:
        exit_time = t
        exit_price = price_t
        exit_reason = "MAX_DURATION"
        BREAK
```

**Parameters:**
| Parameter | Type | Default Range | Description |
|-----------|------|---------------|-------------|
| entry_event_type | enum | {NEW_TOKEN, ACTIVE_TOKEN} | Which trigger to use |
| trail_pct | decimal | {0.05, 0.10, 0.15, 0.20} | Trailing stop percentage |
| initial_stop_pct | decimal | {0.10} | Initial stop loss percentage |
| max_hold_duration | seconds | {3600} | Maximum hold time |

**Outcome Formula:**
```
gross_return = (exit_price - entry_price) / entry_price
outcome = gross_return - total_fee_pct
```

---

### Strategy 3: Event + Liquidity Guard

**Description:**
Enter on event trigger, exit when liquidity drops below threshold.

**Entry:**
```
entry_trigger = NEW_TOKEN or ACTIVE_TOKEN (configurable)
entry_price = trigger_price
entry_time = trigger_time
entry_liquidity = pool_liquidity_at(entry_time)
liquidity_threshold = entry_liquidity * (1 - liquidity_drop_pct)
```

**Exit:**
```
FOR each time t after entry_time:
    current_liquidity = pool_liquidity_at(t)

    IF current_liquidity < liquidity_threshold:
        exit_time = t
        exit_price = price_at(t)
        exit_reason = "LIQUIDITY_DROP"
        BREAK

    IF (t - entry_time) >= max_hold_duration:
        exit_time = t
        exit_price = price_at(t)
        exit_reason = "MAX_DURATION"
        BREAK
```

**Parameters:**
| Parameter | Type | Default Range | Description |
|-----------|------|---------------|-------------|
| entry_event_type | enum | {NEW_TOKEN, ACTIVE_TOKEN} | Which trigger to use |
| liquidity_drop_pct | decimal | {0.20, 0.30, 0.50} | Liquidity drop threshold |
| max_hold_duration | seconds | {1800} | Maximum hold time |

**Outcome Formula:**
```
gross_return = (exit_price - entry_price) / entry_price
outcome = gross_return - total_fee_pct
```

---

## Parameter Sweep Configuration

For simulation, run all combinations:

```
entry_types = [NEW_TOKEN, ACTIVE_TOKEN]

time_exit_params = {
    hold_duration: [60, 300, 600, 1800]
}

trailing_stop_params = {
    trail_pct: [0.05, 0.10, 0.15, 0.20],
    initial_stop_pct: [0.10],
    max_hold_duration: [3600]
}

liquidity_guard_params = {
    liquidity_drop_pct: [0.20, 0.30, 0.50],
    max_hold_duration: [1800]
}
```

**Total Configurations:**
- Time Exit: 2 × 4 = 8
- Trailing Stop: 2 × 4 × 1 × 1 = 8
- Liquidity Guard: 2 × 3 × 1 = 6
- **Total: 22 configurations per scenario**

---

## References
- `docs/EXECUTION_SCENARIOS.md` — execution parameter scenarios
- `docs/MVP_CRITERIA.md` — simulation requirements
- `docs/BRD.md` — strategy requirements
