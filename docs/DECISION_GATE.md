# Decision Gate - Phase 1 GO/NO-GO

## Purpose
- Binary GO/NO-GO decision for Phase 1
- No subjective interpretation allowed
- Decision must be reproducible from data and strategy version

---

## 1. Data Sufficiency Checklist

Before evaluation, all items must be verified:

| # | Criterion | Required | Actual | Pass |
|---|-----------|----------|--------|------|
| 1 | Unique NEW_TOKEN candidates discovered | >=300 | ___ | [ ] |
| 2 | Continuous discovery uptime | >=7 days | ___ | [ ] |
| 3 | Data available for backtest | >=14 days | ___ | [ ] |
| 4 | Duplicate candidate_id count | 0 | ___ | [ ] |
| 5 | Missing events in evaluation period | 0 | ___ | [ ] |
| 6 | Tokens fully replayable from stored data | 100% | ___ | [ ] |

**If any item fails -> STOP. Insufficient data for decision.**

---

## 2. Required Inputs

### Metrics (per strategy, per scenario)
- `win_rate`: percentage of trades with positive outcome
- `median_outcome`: median return across all trades
- `quantiles`: P10, P25, P50, P75, P90 of outcome distribution
- `max_drawdown`: maximum peak-to-trough decline
- `sensitivity_score`: outcome change ratio between Realistic and Pessimistic scenarios

### Data Context
- `token_count`: number of tokens evaluated
- `time_range`: start and end dates of evaluation data
- `replay_commit_hash`: git commit for reproducibility
- `strategy_version`: version identifier of strategy implementation

### Scenario Matrix Results
Results must be provided for all scenarios:
- Optimistic
- Realistic
- Pessimistic
- Degraded

---

## 3. GO Criteria

**ALL must be true under Realistic scenario:**

| # | Criterion | Threshold | Actual | Pass |
|---|-----------|-----------|--------|------|
| 1 | Tokens with positive outcome | >=5% | ___ | [ ] |
| 2 | Median outcome | >0 | ___ | [ ] |
| 3 | Result stable under parameter degradation | per MVP criteria | ___ | [ ] |
| 4 | Result not driven by outliers | quantiles reported and checked | ___ | [ ] |
| 5 | Entry/exit events implementable in realtime | per MVP criteria | ___ | [ ] |

---

## 4. NO-GO Criteria

**ANY triggers NO-GO:**

| # | Criterion | Trigger Condition | Actual | Triggered |
|---|-----------|-------------------|--------|-----------|
| 1 | Tokens with positive outcome | <5% | ___ | [ ] |
| 2 | Median outcome | <=0 | ___ | [ ] |
| 3 | Edge under small degradation | disappears (per MVP criteria) | ___ | [ ] |
| 4 | Entry implementation | impossible to implement honestly (per MVP criteria) | ___ | [ ] |

---

## 5. Decision Procedure

```
Step 1: Verify Data Sufficiency Checklist
        -> If any item unchecked -> STOP (insufficient data)

Step 2: Run all strategies against Realistic scenario
        -> Record metrics for each strategy

Step 3: Check GO criteria
        -> If all pass -> proceed to Step 4
        -> If any fails -> result = NO-GO

Step 4: Check NO-GO criteria
        -> If any triggers -> result = NO-GO
        -> If none triggers -> result = GO

Step 5: Record decision
        -> Metrics table (all values filled)
        -> Replay link (commit hash)
        -> Strategy version
        -> Scenario used
        -> Date and evaluator
```

---

## 6. Decision Record Template

```
Date: ___
Evaluator: ___
Strategy Version: ___
Replay Commit: ___

Data Sufficiency: PASS / FAIL
GO Criteria: ___ / 5 passed
NO-GO Triggers: ___ / 4 triggered

DECISION: GO / NO-GO

Justification:
- [metric 1]: [value]
- [metric 2]: [value]
- ...

Replay Command:
  git checkout [commit] && go run cmd/replay/main.go --config [config]
```

---

## 7. Auto-Generated Report

The decision evaluation is automated via CLI:

```bash
go run cmd/report/main.go --output-dir=docs
```

This generates `DECISION_GATE_REPORT.md` with the following structure:

```
# Phase 1 Decision Gate Report

Generated at: YYYY-MM-DD HH:MM:SS UTC

## Strategy: [STRATEGY_ID] | [ENTRY_EVENT_TYPE]

# Decision Gate Report

## Decision: GO | NO-GO

## GO Criteria

| # | Criterion | Threshold | Actual | Pass |
|---|-----------|-----------|--------|------|
| 1 | Positive outcome tokens | >= 5% | X.XX% | PASS/FAIL |
| 2 | Median outcome | > 0 | X.XXXX | PASS/FAIL |
| 3 | Stable under degradation | ... | ... | PASS/FAIL |
| 4 | Not dominated by outliers | P25 > 0 | X.XXXX | PASS/FAIL |
| 5 | Entry/exit implementable | true | true/false | PASS/FAIL |

GO Criteria: N/5 passed

## NO-GO Triggers

| # | Trigger | Condition | Actual | Status |
|---|---------|-----------|--------|--------|
| 1 | Low positive outcome | < 5% | X.XX% | TRIGGERED/NOT TRIGGERED |
| 2 | Negative/zero median | <= 0 | X.XXXX | TRIGGERED/NOT TRIGGERED |
| 3 | Edge disappears | ... | ... | TRIGGERED/NOT TRIGGERED |
| 4 | Entry not implementable | false | true/false | TRIGGERED/NOT TRIGGERED |

NO-GO Triggers: N/4 triggered

## Summary

[Reasons for decision]
```

The report is deterministic: same inputs produce identical outputs.
See `docs/PIPELINE.md` for details.

---

## References
- `docs/MVP_CRITERIA.md` - authoritative thresholds
- `docs/BRD.md` - business requirements
- `docs/EXECUTION_SCENARIOS.md` - scenario definitions
- `docs/PIPELINE.md` - pipeline architecture and determinism
