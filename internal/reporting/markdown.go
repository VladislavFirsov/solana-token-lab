package reporting

import (
	"fmt"
	"strings"
	"time"
)

// RenderMarkdown renders report as Markdown string.
func RenderMarkdown(r *Report) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Phase 1 Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Strategies: %d | Scenarios: %d\n\n", r.StrategyCount, r.ScenarioCount))

	// Data Summary
	sb.WriteString("## Data Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total Candidates | %d |\n", r.DataSummary.TotalCandidates))
	sb.WriteString(fmt.Sprintf("| NEW_TOKEN Candidates | %d |\n", r.DataSummary.NewTokenCandidates))
	sb.WriteString(fmt.Sprintf("| ACTIVE_TOKEN Candidates | %d |\n", r.DataSummary.ActiveTokenCandidates))
	sb.WriteString(fmt.Sprintf("| Total Trades | %d |\n", r.DataSummary.TotalTrades))
	sb.WriteString(fmt.Sprintf("| Date Range Start (ms) | %d |\n", r.DataSummary.DateRangeStart))
	sb.WriteString(fmt.Sprintf("| Date Range End (ms) | %d |\n", r.DataSummary.DateRangeEnd))
	sb.WriteString("\n")

	// Data Quality
	sb.WriteString("## Data Quality\n\n")
	if len(r.DataQuality.SufficiencyChecks) > 0 {
		sb.WriteString("### Sufficiency Checks\n\n")
		sb.WriteString("| Check | Threshold | Actual | Status |\n")
		sb.WriteString("|-------|-----------|--------|--------|\n")
		for _, check := range r.DataQuality.SufficiencyChecks {
			status := "FAIL"
			if check.Pass {
				status = "PASS"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				check.Name, check.Threshold, check.Actual, status))
		}
		sb.WriteString("\n")

		// Overall status
		if r.DataQuality.AllChecksPassed {
			sb.WriteString("**All checks passed.** Proceeding with GO/NO-GO evaluation.\n\n")
		} else {
			sb.WriteString("**Some checks failed.** Decision: INSUFFICIENT_DATA\n\n")
		}
	} else if len(r.DataQuality.IntegrityErrors) == 0 {
		sb.WriteString("No data quality checks performed.\n\n")
	}

	// Integrity errors (always shown if present, even without sufficiency checks)
	if len(r.DataQuality.IntegrityErrors) > 0 {
		sb.WriteString("### Integrity Errors\n\n")
		for _, err := range r.DataQuality.IntegrityErrors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
		sb.WriteString("\n")
	}

	// Strategy Metrics
	sb.WriteString("## Strategy Metrics\n\n")
	if len(r.StrategyMetrics) > 0 {
		sb.WriteString("| Strategy | Scenario | Entry | Trades | Tokens | WinRate | TokenWinRate | Mean | Median | P10 | P90 | MaxDD | MaxLoss |\n")
		sb.WriteString("|----------|----------|-------|--------|--------|---------|--------------|------|--------|-----|-----|-------|--------|\n")
		for _, m := range r.StrategyMetrics {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %d | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %d |\n",
				m.StrategyID, m.ScenarioID, m.EntryEventType,
				m.TotalTrades, m.TotalTokens, m.WinRate, m.TokenWinRate, m.OutcomeMean, m.OutcomeMedian,
				m.OutcomeP10, m.OutcomeP90, m.MaxDrawdown, m.MaxConsecutiveLosses))
		}
	} else {
		sb.WriteString("No strategy metrics available.\n")
	}
	sb.WriteString("\n")

	// Source Comparison
	sb.WriteString("## NEW_TOKEN vs ACTIVE_TOKEN Comparison\n\n")
	if len(r.SourceComparison) > 0 {
		sb.WriteString("| Strategy | Scenario | NEW WinRate | ACTIVE WinRate | NEW Median | ACTIVE Median |\n")
		sb.WriteString("|----------|----------|-------------|----------------|------------|---------------|\n")
		for _, c := range r.SourceComparison {
			sb.WriteString(fmt.Sprintf("| %s | %s | %.4f | %.4f | %.4f | %.4f |\n",
				c.StrategyID, c.ScenarioID,
				c.NewTokenWinRate, c.ActiveTokenWinRate,
				c.NewTokenMedian, c.ActiveTokenMedian))
		}
	} else {
		sb.WriteString("No source comparison available.\n")
	}
	sb.WriteString("\n")

	// Scenario Sensitivity
	sb.WriteString("## Scenario Sensitivity\n\n")
	if len(r.ScenarioSensitivity) > 0 {
		sb.WriteString("| Strategy | Entry | Realistic | Pessimistic | Degraded | Degradation% |\n")
		sb.WriteString("|----------|-------|-----------|-------------|----------|-------------|\n")
		for _, s := range r.ScenarioSensitivity {
			sb.WriteString(fmt.Sprintf("| %s | %s | %.4f | %.4f | %.4f | %.2f |\n",
				s.StrategyID, s.EntryEventType,
				s.RealisticMean, s.PessimisticMean, s.DegradedMean, s.DegradationPct))
		}
	} else {
		sb.WriteString("No scenario sensitivity data available.\n")
	}
	sb.WriteString("\n")

	// Replay References
	sb.WriteString("## Replay References\n\n")
	if len(r.ReplayReferences) > 0 {
		sb.WriteString("| Strategy | Scenario | Candidate |\n")
		sb.WriteString("|----------|----------|----------|\n")
		for _, ref := range r.ReplayReferences {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				ref.StrategyID, ref.ScenarioID, ref.CandidateID))
		}
	} else {
		sb.WriteString("No replay references available.\n")
	}
	sb.WriteString("\n")

	return sb.String()
}
