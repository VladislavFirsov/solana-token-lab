package strategy

import (
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/idhash"
)

// priceAt returns price at or before target timestamp.
// Per REPLAY_PROTOCOL.md: find closest price at or before target_time.
// If no price before target, returns first available price.
func priceAt(target int64, prices []*domain.PriceTimeseriesPoint) float64 {
	if len(prices) == 0 {
		return 0
	}

	// Find closest price at or before target
	for i := len(prices) - 1; i >= 0; i-- {
		if prices[i].TimestampMs <= target {
			return prices[i].Price
		}
	}

	// If no price before target, use first available
	return prices[0].Price
}

// liquidityAt returns liquidity at or before target timestamp.
// Per REPLAY_PROTOCOL.md: find closest liquidity at or before target_time.
// Returns nil if no liquidity event before target.
func liquidityAt(target int64, liq []*domain.LiquidityTimeseriesPoint) *float64 {
	if len(liq) == 0 {
		return nil
	}

	// Find closest liquidity at or before target
	for i := len(liq) - 1; i >= 0; i-- {
		if liq[i].TimestampMs <= target {
			return &liq[i].Liquidity
		}
	}

	// If no liquidity event before target, return nil
	return nil
}

// applyEntryExecution applies scenario parameters to entry.
// Per SIMULATION_SPEC.md:
//   - entry_time_actual = signal_time + delay_ms
//   - entry_price_actual = signal_price * (1 + slippage_pct / 200)
//   - entry_cost = fee_sol + priority_fee_sol
func applyEntryExecution(signalTime int64, signalPrice float64, scenario domain.ScenarioConfig) (actualTime int64, actualPrice float64, cost float64) {
	actualTime = signalTime + scenario.DelayMs
	actualPrice = signalPrice * (1 + scenario.SlippagePct/200)
	cost = scenario.FeeSOL + scenario.PriorityFeeSOL
	return
}

// applyExitExecution applies scenario parameters to exit.
// Per SIMULATION_SPEC.md:
//   - exit_time_actual = signal_time + delay_ms
//   - exit_price_actual = signal_price * (1 - slippage_pct / 200)
//   - exit_cost = fee_sol + priority_fee_sol
func applyExitExecution(signalTime int64, signalPrice float64, scenario domain.ScenarioConfig) (actualTime int64, actualPrice float64, cost float64) {
	actualTime = signalTime + scenario.DelayMs
	actualPrice = signalPrice * (1 - scenario.SlippagePct/200)
	cost = scenario.FeeSOL + scenario.PriorityFeeSOL
	return
}

// computeTradeID generates deterministic trade ID.
// Delegates to idhash.ComputeTradeID for single-source-of-truth.
func computeTradeID(candidateID, strategyID, scenarioID string, entrySignalTime int64) string {
	return idhash.ComputeTradeID(candidateID, strategyID, scenarioID, entrySignalTime)
}

// buildTradeRecord constructs a complete TradeRecord from execution details.
func buildTradeRecord(
	candidateID, strategyID, scenarioID string,
	entrySignalTime int64, entrySignalPrice float64, entryLiquidity *float64,
	exitSignalTime int64, exitSignalPrice float64, exitReason string,
	scenario domain.ScenarioConfig,
	peakPrice *float64, minLiquidity *float64,
) *domain.TradeRecord {
	// Apply entry execution
	entryActualTime, entryActualPrice, entryCost := applyEntryExecution(entrySignalTime, entrySignalPrice, scenario)

	// Apply exit execution
	exitActualTime, exitActualPrice, exitCost := applyExitExecution(exitSignalTime, exitSignalPrice, scenario)

	// Calculate position
	positionSize := 1.0
	positionValue := entryActualPrice * positionSize

	// Calculate MEV cost
	mevCost := positionValue * (scenario.MEVPenaltyPct / 100)

	// Calculate total costs
	totalCost := entryCost + exitCost + mevCost
	totalCostPct := totalCost / positionValue

	// Calculate outcome
	grossReturn := (exitActualPrice - entryActualPrice) / entryActualPrice
	outcome := grossReturn - totalCostPct

	// Classify outcome
	outcomeClass := domain.OutcomeClassLoss
	if outcome > 0 {
		outcomeClass = domain.OutcomeClassWin
	}

	// Calculate hold duration
	holdDurationMs := exitActualTime - entryActualTime

	// Generate trade ID
	tradeID := computeTradeID(candidateID, strategyID, scenarioID, entrySignalTime)

	return &domain.TradeRecord{
		TradeID:     tradeID,
		CandidateID: candidateID,
		StrategyID:  strategyID,
		ScenarioID:  scenarioID,

		EntrySignalTime:  entrySignalTime,
		EntrySignalPrice: entrySignalPrice,
		EntryActualTime:  entryActualTime,
		EntryActualPrice: entryActualPrice,
		EntryLiquidity:   entryLiquidity,
		PositionSize:     positionSize,
		PositionValue:    positionValue,

		ExitSignalTime:  exitSignalTime,
		ExitSignalPrice: exitSignalPrice,
		ExitActualTime:  exitActualTime,
		ExitActualPrice: exitActualPrice,
		ExitReason:      exitReason,

		EntryCostSOL: entryCost,
		ExitCostSOL:  exitCost,
		MEVCostSOL:   mevCost,
		TotalCostSOL: totalCost,
		TotalCostPct: totalCostPct,

		GrossReturn:  grossReturn,
		Outcome:      outcome,
		OutcomeClass: outcomeClass,

		HoldDurationMs: holdDurationMs,
		PeakPrice:      peakPrice,
		MinLiquidity:   minLiquidity,
	}
}
