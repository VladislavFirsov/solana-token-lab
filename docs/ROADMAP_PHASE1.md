# Roadmap - Phase 1

## Overview
- **Goal:** Reproducible GO/NO-GO decision for Solana token trading edge
- **Approach:** Sequential milestones with measurable gates
- **Principle:** No milestone starts until previous is verified complete

---

## Milestones

### M1: Discovery (NEW_TOKEN)

**Entry Criteria:**
- RPC connection to Solana established
- pump.fun program ID identified
- PostgreSQL database initialized

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Unique tokens discovered | >=300 | `SELECT COUNT(DISTINCT candidate_id) FROM tokens WHERE source = 'NEW_TOKEN'` |
| Continuous operation | >=7 days | Uptime logs, no gaps >1 hour |
| Duplicate candidate_id | 0 | `SELECT candidate_id, COUNT(*) FROM tokens GROUP BY candidate_id HAVING COUNT(*) > 1` |
| Fields captured | mint, pool, tx_signature, slot, timestamp | Schema validation |

**Verification Method:**
- SQL queries return expected values
- Log review confirms continuous operation

---

### M2: Discovery (ACTIVE_TOKEN)

**Entry Criteria:**
- M1 complete
- Volume/swap baseline data available (>=24h history)

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| ACTIVE_TOKEN stream | Producing candidates | Query returns results |
| Spike formula documented | `volume_1h > 3 * volume_24h_avg OR swaps_1h > 5 * swaps_24h_avg` | Code review |
| Unified format | Same schema as NEW_TOKEN | Schema comparison |

**Verification Method:**
- SQL query validates spike formula for all ACTIVE_TOKEN candidates:
  ```sql
  SELECT candidate_id, volume_1h, volume_24h_avg, swaps_1h, swaps_24h_avg
  FROM active_token_candidates
  WHERE NOT (volume_1h > 3 * volume_24h_avg OR swaps_1h > 5 * swaps_24h_avg)
  -- Must return 0 rows (all candidates match formula)
  ```
- Schema comparison query confirms format matches NEW_TOKEN

---

### M3: Ingestion

**Entry Criteria:**
- M1 + M2 complete
- Token candidates flowing into system

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Event types stored | swaps, liquidity_add, liquidity_remove | Schema check |
| Operation without event loss | >=7 days | Compare DB count vs RPC query for sample tokens |
| Manual restarts required | 0 | Operations log |
| Storage type | Append-only PostgreSQL | Architecture review |

**Verification Method:**
- Select 5 random tokens
- Query RPC for all events
- Compare with stored events (count must match)

---

### M4: Normalization & Features

**Entry Criteria:**
- M3 complete
- Raw event data available in PostgreSQL

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Time series generated | price_t, liquidity_t, volume_t | Query returns data |
| Derived features | delta, velocity, acceleration, token_lifetime, event_intervals | Feature table exists |
| Formulas documented | All in code comments or docs | Code review |
| Heuristics used | 0 | Code review (no magic numbers without formula) |

**Verification Method:**
- Run normalization twice on same token
- Output must be identical (deterministic)

---

### M5: Storage & Replay

**Entry Criteria:**
- M4 complete
- Normalized data available

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| PostgreSQL as source of truth | Yes | Architecture diagram |
| ClickHouse connected | Yes | Connection test |
| Token replayability | Any token fully replayable | Test replay command |
| Analysis data source | DB only, not RPC | Code review |

**Verification Method:**
- Replay same token twice
- Compare output byte-by-byte
- Must be identical

---

### M6: Simulation / Backtest

**Entry Criteria:**
- M5 complete
- >=14 days of data available

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Strategies implemented | Time Exit, Trailing Stop, Liquidity Guard | Code exists |
| Scenarios run | Optimistic, Realistic, Pessimistic, Degraded | Results for each |
| Result format | Distributions, not single values | Output contains quantiles |

**Verification Method:**
- Vary parameters for one strategy
- Results must change (not hardcoded)
- Distribution plots generated

---

### M7: Metrics

**Entry Criteria:**
- M6 complete
- Simulation results available

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Metrics computed | win-rate, median, quantiles (P10-P90), max drawdown, sensitivity | Output contains all |
| Comparisons | NEW_TOKEN vs ACTIVE_TOKEN, strategy vs strategy | Comparison tables exist |
| Manual interpretation | None required | Metrics are numeric only |

**Verification Method:**
- Run metrics computation
- All values are numbers (no text conclusions)
- Comparisons are automated

---

### M8: Reporting

**Entry Criteria:**
- M7 complete
- All metrics available

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Report contents | Data description, metrics, replay links, strategy version | Document review |
| Readability | Understandable without author | Third-party review |
| Conclusions | Backed by numbers only | No subjective statements |

**Verification Method:**
- Give report to third party
- They can reproduce results using replay link
- No clarification needed from author

**CLI:**
```bash
go run cmd/report/main.go --output-dir=docs
```
Generates `REPORT_PHASE1.md` and `STRATEGY_AGGREGATES.csv`.

---

### M9: Decision Gate Execution

**Entry Criteria:**
- M8 complete
- Report finalized

**Exit Criteria:**
| Criterion | Required | Verification |
|-----------|----------|--------------|
| Decision made | GO or NO-GO | Recorded in DECISION_GATE.md |
| Justification | DECISION_GATE.md checklist completed | All fields filled |
| Reproducibility | Decision can be re-derived from data | Replay test |

**Verification Method:**
- Present decision to CTO
- Defend with numbers only
- CTO can verify independently

**CLI:**
The same `cmd/report` command also generates `DECISION_GATE_REPORT.md` with GO/NO-GO evaluation per strategy.

---

## Milestone Dependencies

```
M1 --> M2 --> M3 --> M4 --> M5 --> M6 --> M7 --> M8 --> M9
         \----------------------------------------------->
                    (data collection continues in parallel)
```

---

## References
- `docs/MVP_CRITERIA.md` - acceptance criteria per component
- `docs/BRD.md` - business requirements
- `docs/DECISION_GATE.md` - final decision process
