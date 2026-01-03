# Business Requirements — Solana Token Research Platform (Phase 1)

## Business Objective
- Determine if there is a repeatable, implementable edge for trading high-risk Solana tokens (including pump.fun and analogs) that justifies building an automated trading service.
- Phase 1 is a binary decision gate:
  - GO — proceed to live tests and automation
  - NO-GO — stop or pivot

## Scope
### In Scope
- Collect and store Solana on-chain data.
- Analyze token behavior for:
  - New tokens (mint/new pools)
  - Existing tokens with activity spikes
- Deterministic simulation of trading strategies.
- Reporting and metrics for decision-making.
- Reproducible replay analysis.

### Out of Scope
- Real trading or transaction submission.
- ML/AI/LLM decision-making.
- Production-grade scaling or fault tolerance.
- End-user UI.
- Profitability guarantees.

## Non-Functional Business Constraints
- Falsifiability: system must be able to prove strategies do not work.
- Determinism: same input yields same result.
- Research separated from trading: no real-time trade decisions in Phase 1.
- Anti-self-deception: results cannot depend on subjective interpretation.

## Users
- Primary: Research Owner (business/product; not a trader or analyst).
- Secondary: CTO/dev team; future trading operator (Phase 2).

## Functional Requirements
### Discovery
- Detect:
  - New tokens (mint/new pool/first swap)
  - Existing tokens with activity spikes
- Classify discovery source:
  - `NEW_TOKEN`
  - `ACTIVE_TOKEN`
- Output stream: `TokenCandidate`.

### Data Collection
For each `TokenCandidate` collect:
- Swap transactions (time, price, volume).
- Liquidity state over time.
- Add/remove liquidity events.
- Basic on-chain token metadata.

Requirement: append-only storage for replay.

### Normalization & Features
Compute deterministic features including:
- Price and liquidity time series.
- Rate and acceleration of changes.
- Token lifetime.
- Intervals between key events.

Prohibited:
- Heuristics without formula.
- "Estimated"/subjective features.
- ML-based features.

### Strategy Simulation (Backtest)
Supported strategies (minimum set):
- Event + Time Exit
- Event + Trailing Stop
- Event + Liquidity Guard

Required parameters:
- Execution delay (multiple scenarios).
- Slippage (multiple scenarios).
- Fees/priority fee (scenario-based).
- Execution penalty (MEV-penalty as parameter).

Output: statistics per strategy and parameter set.

### Metrics & Reporting
Report must include:
- Win-rate.
- Median/mean outcome.
- Max drawdown.
- Result distributions.
- Sensitivity to parameter degradation.
- Comparison by token class (New vs Active).

Prohibited:
- Textual conclusions without numbers.
- Automated recommendations.

## Decision Gate (GO / NO-GO)
### GO Criteria
- устойчивый положительный outcome, не основанный на выбросах;
- результат сохраняется при ухудшении параметров;
- четко формализуемые entry-события.

### NO-GO Criteria
- отсутствуют устойчивые паттерны;
- полная деградация при реалистичных сценариях;
- edge исчезает при минимальных изменениях условий.

## Non-Functional Requirements
- Reproducibility: any report must be reproducible by data and strategy version.
- Observability: log entry/exit reasons, replay any trade.
- Performance: near-real-time ingestion is OK; ultra-low latency not required.

## Technology Constraints (Recommended)
- Language: Go
- Solana access: RPC (HTTP + WS)
- Storage:
  - Raw data: PostgreSQL or ClickHouse
  - Aggregates: ClickHouse
- Visualization: SQL, Grafana, CSV
- AI/LLM in core is prohibited.

## Risks & Limitations
- Edge may only exist in simulation.
- Real execution can erase the result.
- Negative research outcome is valid.

## Phase 1 Done Criteria
- Reproducible report prepared.
- GO/NO-GO decision made.
- Decision justified by numbers (not interpretation).

## MVP Criteria Reference
See `docs/MVP_CRITERIA.md` for the authoritative Phase 1 MVP readiness criteria used for task decomposition and acceptance.
