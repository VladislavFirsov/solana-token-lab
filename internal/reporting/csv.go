package reporting

import (
	"fmt"
	"strings"

	"solana-token-lab/internal/domain"
)

// csvQuote wraps string in double quotes and escapes internal quotes.
// Per REPORTING_SPEC.md: "Quote: double-quote for strings".
func csvQuote(s string) string {
	escaped := strings.ReplaceAll(s, `"`, `""`)
	return `"` + escaped + `"`
}

// RenderStrategyAggregatesCSV renders strategy aggregates as CSV string.
// Per REPORTING_SPEC.md: 18 columns.
func RenderStrategyAggregatesCSV(metrics []StrategyMetricRow) string {
	var sb strings.Builder

	// Header (18 columns per spec)
	sb.WriteString("strategy_id,scenario_id,entry_event_type,total_trades,wins,losses,win_rate,")
	sb.WriteString("outcome_mean,outcome_median,outcome_p10,outcome_p25,outcome_p75,outcome_p90,")
	sb.WriteString("outcome_min,outcome_max,outcome_stddev,max_drawdown,max_consecutive_losses\n")

	// Rows
	for _, m := range metrics {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%d,%d,%d,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%d\n",
			csvQuote(m.StrategyID),
			csvQuote(m.ScenarioID),
			csvQuote(m.EntryEventType),
			m.TotalTrades,
			m.Wins,
			m.Losses,
			m.WinRate,
			m.OutcomeMean,
			m.OutcomeMedian,
			m.OutcomeP10,
			m.OutcomeP25,
			m.OutcomeP75,
			m.OutcomeP90,
			m.OutcomeMin,
			m.OutcomeMax,
			m.OutcomeStddev,
			m.MaxDrawdown,
			m.MaxConsecutiveLosses,
		))
	}

	return sb.String()
}

// RenderTradeRecordsCSV renders trade records as CSV string.
// Per REPORTING_SPEC.md: 27 columns.
func RenderTradeRecordsCSV(trades []*domain.TradeRecord) string {
	var sb strings.Builder

	// Header (27 columns per spec)
	sb.WriteString("trade_id,candidate_id,strategy_id,scenario_id,")
	sb.WriteString("entry_signal_time,entry_signal_price,entry_actual_time,entry_actual_price,")
	sb.WriteString("entry_liquidity,position_size,position_value,")
	sb.WriteString("exit_signal_time,exit_signal_price,exit_actual_time,exit_actual_price,")
	sb.WriteString("exit_reason,entry_cost_sol,exit_cost_sol,mev_cost_sol,total_cost_sol,")
	sb.WriteString("total_cost_pct,gross_return,outcome,outcome_class,")
	sb.WriteString("hold_duration_ms,peak_price,min_liquidity\n")

	// Rows
	for _, t := range trades {
		// Handle nullable fields
		entryLiquidity := ""
		if t.EntryLiquidity != nil {
			entryLiquidity = fmt.Sprintf("%.6f", *t.EntryLiquidity)
		}
		peakPrice := ""
		if t.PeakPrice != nil {
			peakPrice = fmt.Sprintf("%.6f", *t.PeakPrice)
		}
		minLiquidity := ""
		if t.MinLiquidity != nil {
			minLiquidity = fmt.Sprintf("%.6f", *t.MinLiquidity)
		}

		sb.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d,%.6f,%d,%.6f,%s,%.6f,%.6f,%d,%.6f,%d,%.6f,%s,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%s,%d,%s,%s\n",
			csvQuote(t.TradeID),
			csvQuote(t.CandidateID),
			csvQuote(t.StrategyID),
			csvQuote(t.ScenarioID),
			t.EntrySignalTime,
			t.EntrySignalPrice,
			t.EntryActualTime,
			t.EntryActualPrice,
			entryLiquidity,
			t.PositionSize,
			t.PositionValue,
			t.ExitSignalTime,
			t.ExitSignalPrice,
			t.ExitActualTime,
			t.ExitActualPrice,
			csvQuote(t.ExitReason),
			t.EntryCostSOL,
			t.ExitCostSOL,
			t.MEVCostSOL,
			t.TotalCostSOL,
			t.TotalCostPct,
			t.GrossReturn,
			t.Outcome,
			csvQuote(t.OutcomeClass),
			t.HoldDurationMs,
			peakPrice,
			minLiquidity,
		))
	}

	return sb.String()
}

// RenderScenarioOutcomesCSV renders scenario outcomes as CSV string.
// Per REPORTING_SPEC.md: 6 columns.
func RenderScenarioOutcomesCSV(sensitivity []ScenarioSensitivityRow) string {
	var sb strings.Builder

	// Header (6 columns per spec)
	sb.WriteString("strategy_id,entry_event_type,outcome_optimistic,outcome_realistic,outcome_pessimistic,outcome_degraded\n")

	// Rows
	for _, s := range sensitivity {
		sb.WriteString(fmt.Sprintf("%s,%s,%.6f,%.6f,%.6f,%.6f\n",
			csvQuote(s.StrategyID),
			csvQuote(s.EntryEventType),
			s.OptimisticMedian,
			s.RealisticMedian,
			s.PessimisticMedian,
			s.DegradedMedian,
		))
	}

	return sb.String()
}

// RenderCSV is deprecated - use RenderStrategyAggregatesCSV instead.
// Kept for backwards compatibility.
func RenderCSV(metrics []StrategyMetricRow) string {
	return RenderStrategyAggregatesCSV(metrics)
}
