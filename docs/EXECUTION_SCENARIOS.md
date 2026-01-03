# Execution Scenarios — Phase 1

## Purpose
- Model real-world execution conditions for strategy simulation
- Test strategy robustness under parameter degradation
- Prevent overfitting to ideal conditions
- Enable sensitivity analysis for GO/NO-GO decision

---

## Parameter Definitions

### delay_ms
**Description:** Time from signal detection to transaction execution.

**Components:**
- Signal processing latency
- Network round-trip time
- Transaction submission time
- Block inclusion time

**Unit:** milliseconds

---

### slippage_pct
**Description:** Price impact from order execution.

**Components:**
- Market depth impact
- Order size relative to liquidity
- Price movement during execution

**Unit:** percentage of trade value

**Application:**
- Entry: price worsens (pay more)
- Exit: price worsens (receive less)

---

### fee_sol
**Description:** Base Solana transaction fee.

**Components:**
- Signature verification cost
- Compute units consumed

**Unit:** SOL

---

### priority_fee_sol
**Description:** Additional fee for faster block inclusion.

**Components:**
- Priority queue position
- Network congestion premium

**Unit:** SOL

---

### mev_penalty_pct
**Description:** Value extracted by MEV bots.

**Components:**
- Sandwich attack losses
- Frontrunning losses
- Backrunning extraction

**Unit:** percentage of trade value

---

## Scenario Matrix

| Parameter | Optimistic | Realistic | Pessimistic | Degraded |
|-----------|------------|-----------|-------------|----------|
| delay_ms | 100 | 500 | 2000 | 5000 |
| slippage_pct | 0.5 | 2.0 | 5.0 | 10.0 |
| fee_sol | 0.000005 | 0.00001 | 0.0001 | 0.001 |
| priority_fee_sol | 0 | 0.0001 | 0.001 | 0.01 |
| mev_penalty_pct | 0 | 1.0 | 3.0 | 5.0 |

---

## Scenario Definitions

### Optimistic
**Use case:** Upper bound estimation, best-case reference.

**Assumptions:**
- Fast, dedicated infrastructure
- Low network congestion
- Minimal competition
- No MEV extraction

**When to use:**
- Establishing theoretical maximum
- Comparing against realistic expectations

---

### Realistic
**Use case:** Primary evaluation scenario for GO/NO-GO decision.

**Assumptions:**
- Standard cloud infrastructure
- Normal network conditions
- Moderate competition
- Typical MEV presence

**When to use:**
- GO/NO-GO decision is based on this scenario
- Expected production performance

---

### Pessimistic
**Use case:** Robustness testing.

**Assumptions:**
- Network congestion
- High competition
- Elevated MEV activity
- Infrastructure under load

**When to use:**
- Sensitivity analysis
- Stress testing
- Verify result stability under parameter degradation (per MVP criteria)

---

### Degraded
**Use case:** Failure boundary identification.

**Assumptions:**
- Worst-case network conditions
- Maximum competition
- Aggressive MEV extraction
- Infrastructure failures

**When to use:**
- Finding break-even point
- Understanding failure modes
- Edge case analysis

---

## Application Rules

### Entry Execution
```
entry_price_actual = entry_price_signal * (1 + slippage_pct / 100 / 2)
entry_time_actual = signal_time + delay_ms
entry_cost = fee_sol + priority_fee_sol
```

### Exit Execution
```
exit_price_actual = exit_price_signal * (1 - slippage_pct / 100 / 2)
exit_time_actual = exit_signal_time + delay_ms
exit_cost = fee_sol + priority_fee_sol
```

### MEV Penalty
```
mev_cost = position_value * (mev_penalty_pct / 100)
```

### Total Cost
```
total_cost = entry_cost + exit_cost + mev_cost
total_cost_pct = total_cost / position_value
```

### Adjusted Outcome
```
gross_return = (exit_price_actual - entry_price_actual) / entry_price_actual
outcome = gross_return - total_cost_pct
```

---

## Scenario Selection Rules

### Primary Evaluation
- **Scenario:** Realistic
- **Decision basis:** GO/NO-GO determined from Realistic results

### Sensitivity Testing
Compare outcomes across scenarios to assess stability under degradation (per MVP criteria).

Run all strategies against all scenarios and report:
- Outcome distributions per scenario
- How results change between Realistic and Pessimistic
- Where edge breaks down (if applicable)

### Reporting Requirements
All four scenarios must be run and reported:
- Include outcome distributions for each
- Show outcome comparison across scenarios
- Highlight where edge breaks down

---

## Simulation Configuration

```yaml
scenarios:
  optimistic:
    delay_ms: 100
    slippage_pct: 0.5
    fee_sol: 0.000005
    priority_fee_sol: 0
    mev_penalty_pct: 0

  realistic:
    delay_ms: 500
    slippage_pct: 2.0
    fee_sol: 0.00001
    priority_fee_sol: 0.0001
    mev_penalty_pct: 1.0

  pessimistic:
    delay_ms: 2000
    slippage_pct: 5.0
    fee_sol: 0.0001
    priority_fee_sol: 0.001
    mev_penalty_pct: 3.0

  degraded:
    delay_ms: 5000
    slippage_pct: 10.0
    fee_sol: 0.001
    priority_fee_sol: 0.01
    mev_penalty_pct: 5.0
```

---

## References
- `docs/STRATEGY_CATALOG.md` — strategy definitions
- `docs/DECISION_GATE.md` — GO/NO-GO criteria
- `docs/MVP_CRITERIA.md` — simulation requirements
