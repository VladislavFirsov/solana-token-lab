package reporting

import (
	"fmt"
	"strings"
	"time"
)

// RenderMarkdown renders report as Markdown string per REPORTING_SPEC.md.
func RenderMarkdown(r *Report) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Phase 1 Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Strategies: %d | Scenarios: %d\n\n", r.StrategyCount, r.ScenarioCount))

	// Executive Summary (per REPORTING_SPEC.md)
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Decision | %s |\n", r.ExecutiveSummary.Decision))
	if r.ExecutiveSummary.BestStrategy != "" {
		sb.WriteString(fmt.Sprintf("| Best Strategy | %s (%s) |\n", r.ExecutiveSummary.BestStrategy, r.ExecutiveSummary.BestEntryType))
	}
	sb.WriteString(fmt.Sprintf("| Win Rate (Realistic) | %.2f%% |\n", r.ExecutiveSummary.WinRateRealistic*100))
	sb.WriteString(fmt.Sprintf("| Median Outcome (Realistic) | %.4f |\n", r.ExecutiveSummary.MedianRealistic))
	sb.WriteString(fmt.Sprintf("| Median Outcome (Pessimistic) | %.4f |\n", r.ExecutiveSummary.MedianPessimistic))
	if !r.ExecutiveSummary.DataPeriodStart.IsZero() {
		sb.WriteString(fmt.Sprintf("| Data Period | %s to %s |\n",
			r.ExecutiveSummary.DataPeriodStart.Format(time.RFC3339),
			r.ExecutiveSummary.DataPeriodEnd.Format(time.RFC3339)))
	}
	sb.WriteString(fmt.Sprintf("| NEW_TOKEN Candidates | %d |\n", r.ExecutiveSummary.NewTokenCount))
	sb.WriteString(fmt.Sprintf("| ACTIVE_TOKEN Candidates | %d |\n", r.ExecutiveSummary.ActiveTokenCount))
	sb.WriteString("\n")

	// Data Summary with ISO timestamps (per REPORTING_SPEC.md)
	sb.WriteString("## Data Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Total Candidates | %d |\n", r.DataSummary.TotalCandidates))
	sb.WriteString(fmt.Sprintf("| NEW_TOKEN Candidates | %d |\n", r.DataSummary.NewTokenCandidates))
	sb.WriteString(fmt.Sprintf("| ACTIVE_TOKEN Candidates | %d |\n", r.DataSummary.ActiveTokenCandidates))
	sb.WriteString(fmt.Sprintf("| Total Trades | %d |\n", r.DataSummary.TotalTrades))

	// Format timestamps as ISO 8601 and calculate duration
	if r.DataSummary.DateRangeStart > 0 && r.DataSummary.DateRangeEnd > 0 {
		startTime := time.UnixMilli(r.DataSummary.DateRangeStart).UTC()
		endTime := time.UnixMilli(r.DataSummary.DateRangeEnd).UTC()
		duration := endTime.Sub(startTime)
		durationDays := duration.Hours() / 24

		sb.WriteString(fmt.Sprintf("| Date Range Start | %s |\n", startTime.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("| Date Range End | %s |\n", endTime.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("| Duration | %.1f days |\n", durationDays))
	}
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

	// Strategy Metrics with full columns (per REPORTING_SPEC.md)
	sb.WriteString("## Strategy Metrics\n\n")
	if len(r.StrategyMetrics) > 0 {
		sb.WriteString("| Strategy | Scenario | Entry | Trades | Wins | Losses | WinRate | Mean | Median | P10 | P25 | P75 | P90 | Min | Max | Stddev | MaxDD | MaxLoss |\n")
		sb.WriteString("|----------|----------|-------|--------|------|--------|---------|------|--------|-----|-----|-----|-----|-----|-----|--------|-------|--------|\n")
		for _, m := range r.StrategyMetrics {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %d | %d | %d | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f | %d |\n",
				m.StrategyID, m.ScenarioID, m.EntryEventType,
				m.TotalTrades, m.Wins, m.Losses, m.WinRate,
				m.OutcomeMean, m.OutcomeMedian,
				m.OutcomeP10, m.OutcomeP25, m.OutcomeP75, m.OutcomeP90,
				m.OutcomeMin, m.OutcomeMax, m.OutcomeStddev,
				m.MaxDrawdown, m.MaxConsecutiveLosses))
		}
	} else {
		sb.WriteString("No strategy metrics available.\n")
	}
	sb.WriteString("\n")

	// Source Comparison with delta (per REPORTING_SPEC.md: Realistic only)
	sb.WriteString("## NEW_TOKEN vs ACTIVE_TOKEN Comparison (Realistic Scenario)\n\n")
	if len(r.SourceComparison) > 0 {
		sb.WriteString("| Strategy | NEW WinRate | ACTIVE WinRate | Δ WinRate | NEW Median | ACTIVE Median | Δ Median |\n")
		sb.WriteString("|----------|-------------|----------------|-----------|------------|---------------|----------|\n")
		for _, c := range r.SourceComparison {
			sb.WriteString(fmt.Sprintf("| %s | %.4f | %.4f | %.4f | %.4f | %.4f | %.4f |\n",
				c.StrategyID,
				c.NewTokenWinRate, c.ActiveTokenWinRate, c.DeltaWinRate,
				c.NewTokenMedian, c.ActiveTokenMedian, c.DeltaMedian))
		}
	} else {
		sb.WriteString("No source comparison available.\n")
	}
	sb.WriteString("\n")

	// Scenario Sensitivity with median and optimistic (per REPORTING_SPEC.md)
	sb.WriteString("## Scenario Sensitivity (Median Outcomes)\n\n")
	if len(r.ScenarioSensitivity) > 0 {
		sb.WriteString("| Strategy | Entry | Optimistic | Realistic | Pessimistic | Degraded | Δ% (R→P) |\n")
		sb.WriteString("|----------|-------|------------|-----------|-------------|----------|----------|\n")
		for _, s := range r.ScenarioSensitivity {
			sb.WriteString(fmt.Sprintf("| %s | %s | %.4f | %.4f | %.4f | %.4f | %.2f%% |\n",
				s.StrategyID, s.EntryEventType,
				s.OptimisticMedian, s.RealisticMedian, s.PessimisticMedian, s.DegradedMedian,
				s.DegradationPct))
		}
	} else {
		sb.WriteString("No scenario sensitivity data available.\n")
	}
	sb.WriteString("\n")

	// Reproducibility (per REPORTING_SPEC.md)
	sb.WriteString("## Reproducibility\n\n")
	sb.WriteString("| Metadata | Value |\n")
	sb.WriteString("|----------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Report Timestamp | %s |\n", r.Reproducibility.ReportTimestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("| Generator Version | %s |\n", r.Reproducibility.GeneratorVersion))
	sb.WriteString(fmt.Sprintf("| Data Version | %s |\n", r.Reproducibility.DataVersion))
	sb.WriteString(fmt.Sprintf("| Strategy Version | %s |\n", r.Reproducibility.StrategyVersion))
	sb.WriteString(fmt.Sprintf("| Replay Commit | %s |\n", r.Reproducibility.ReplayCommitHash))
	if r.Reproducibility.ReplayCommand != "" {
		sb.WriteString(fmt.Sprintf("| Replay Command | `%s` |\n", r.Reproducibility.ReplayCommand))
	}
	sb.WriteString("\n")

	// Decision Checklist reference
	if r.DecisionChecklistRef != "" {
		sb.WriteString("## Decision Checklist\n\n")
		sb.WriteString(fmt.Sprintf("See: %s\n\n", r.DecisionChecklistRef))
	}

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
