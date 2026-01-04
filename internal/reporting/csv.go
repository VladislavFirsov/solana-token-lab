package reporting

import (
	"fmt"
	"strings"
)

// RenderCSV renders strategy aggregates as CSV string.
func RenderCSV(metrics []StrategyMetricRow) string {
	var sb strings.Builder

	// Header
	sb.WriteString("strategy_id,scenario_id,entry_event_type,total_trades,total_tokens,win_rate,token_win_rate,")
	sb.WriteString("outcome_mean,outcome_median,outcome_p10,outcome_p90,")
	sb.WriteString("max_drawdown,max_consecutive_losses\n")

	// Rows
	for _, m := range metrics {
		sb.WriteString(fmt.Sprintf("%s,%s,%s,%d,%d,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%d\n",
			m.StrategyID,
			m.ScenarioID,
			m.EntryEventType,
			m.TotalTrades,
			m.TotalTokens,
			m.WinRate,
			m.TokenWinRate,
			m.OutcomeMean,
			m.OutcomeMedian,
			m.OutcomeP10,
			m.OutcomeP90,
			m.MaxDrawdown,
			m.MaxConsecutiveLosses,
		))
	}

	return sb.String()
}
