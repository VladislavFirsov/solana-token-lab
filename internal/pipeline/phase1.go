package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/metrics"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/reporting"
	"solana-token-lab/internal/storage"
)

// Phase1Pipeline orchestrates report + decision generation.
type Phase1Pipeline struct {
	reportGen          *reporting.Generator
	decisionBuild      *decision.Builder
	decisionEval       *decision.Evaluator
	sufficiencyChecker *SufficiencyChecker
	aggregator         *metrics.Aggregator // optional, for collecting missing candidate errors
	outputDir          string
	clock              func() time.Time
	integrityErrors    []string // additional integrity errors (e.g., from aggregation)
}

// NewPhase1Pipeline creates a new pipeline.
func NewPhase1Pipeline(
	candidateStore storage.CandidateStore,
	tradeStore storage.TradeRecordStore,
	aggStore storage.StrategyAggregateStore,
	implementable map[decision.StrategyKey]bool,
	outputDir string,
) *Phase1Pipeline {
	return &Phase1Pipeline{
		reportGen:     reporting.NewGenerator(candidateStore, tradeStore, aggStore),
		decisionBuild: decision.NewBuilder(implementable),
		decisionEval:  decision.NewEvaluator(),
		outputDir:     outputDir,
		clock:         func() time.Time { return time.Now().UTC() },
	}
}

// WithSufficiencyChecker adds a sufficiency checker to the pipeline.
func (p *Phase1Pipeline) WithSufficiencyChecker(
	candidateStore storage.CandidateStore,
	tradeStore storage.TradeRecordStore,
	swapStore storage.SwapStore,
	liquidityStore storage.LiquidityEventStore,
	replayRunner *replay.Runner,
) *Phase1Pipeline {
	p.sufficiencyChecker = NewSufficiencyChecker(candidateStore, tradeStore, swapStore, liquidityStore, replayRunner)
	return p
}

// WithClock sets a custom clock function for deterministic output.
func (p *Phase1Pipeline) WithClock(clock func() time.Time) *Phase1Pipeline {
	p.clock = clock
	p.reportGen = p.reportGen.WithClock(clock)
	return p
}

// WithIntegrityErrors adds additional integrity errors to include in the report.
// These are merged with errors from sufficiency checks.
// Use this to pass missing candidate errors from aggregation.
func (p *Phase1Pipeline) WithIntegrityErrors(errors []string) *Phase1Pipeline {
	p.integrityErrors = append(p.integrityErrors, errors...)
	return p
}

// WithAggregator sets the aggregator to automatically collect missing candidate errors.
// The aggregator's MissingCandidates are collected during Run() and merged with integrity errors.
// This is the preferred way to wire aggregator errors - call this after computing aggregates.
func (p *Phase1Pipeline) WithAggregator(agg *metrics.Aggregator) *Phase1Pipeline {
	p.aggregator = agg
	return p
}

// Run executes full pipeline and writes output files:
// - REPORT_PHASE1.md
// - STRATEGY_AGGREGATES.csv
// - DECISION_GATE_REPORT.md
func (p *Phase1Pipeline) Run(ctx context.Context) error {
	// Ensure output directory exists
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return err
	}

	// 1. Run sufficiency check FIRST (if configured)
	var dataQuality reporting.DataQualitySection
	if p.sufficiencyChecker != nil {
		suffResult, err := p.sufficiencyChecker.Check(ctx)
		if err != nil {
			return err
		}
		dataQuality = convertToDataQuality(suffResult)
	}

	// Collect missing candidate errors from aggregator (if configured)
	if p.aggregator != nil {
		aggErrors := p.aggregator.GetMissingCandidateErrors()
		if len(aggErrors) > 0 {
			p.integrityErrors = append(p.integrityErrors, aggErrors...)
		}
	}

	// Merge additional integrity errors (e.g., from aggregation)
	if len(p.integrityErrors) > 0 {
		dataQuality.IntegrityErrors = append(dataQuality.IntegrityErrors, p.integrityErrors...)
		// If we have integrity errors, all checks did not pass
		dataQuality.AllChecksPassed = false
	}

	// 2. Generate report (includes data quality section)
	report, err := p.reportGen.Generate(ctx)
	if err != nil {
		return err
	}
	report.DataQuality = dataQuality

	// 3. Write REPORT_PHASE1.md
	reportMD := reporting.RenderMarkdown(report)
	reportPath := filepath.Join(p.outputDir, "REPORT_PHASE1.md")
	if err := os.WriteFile(reportPath, []byte(reportMD), 0644); err != nil {
		return err
	}

	// 4. Write STRATEGY_AGGREGATES.csv
	csvContent := reporting.RenderCSV(report.StrategyMetrics)
	csvPath := filepath.Join(p.outputDir, "STRATEGY_AGGREGATES.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		return err
	}

	// 5. If sufficiency fails -> INSUFFICIENT_DATA decision
	if p.sufficiencyChecker != nil && !dataQuality.AllChecksPassed {
		return p.writeInsufficientDataReport(dataQuality)
	}

	// 6. Otherwise proceed with GO/NO-GO evaluation
	inputs, err := p.decisionBuild.BuildAll(report)
	if err != nil {
		// If no realistic scenarios, still write empty decision report
		if err == decision.ErrNoRealisticScenario {
			decisionPath := filepath.Join(p.outputDir, "DECISION_GATE_REPORT.md")
			content := "# Decision Gate Report\n\nNo realistic scenario data available.\n"
			return os.WriteFile(decisionPath, []byte(content), 0644)
		}
		return err
	}

	// Evaluate each strategy and render combined decision report
	decisionMD, err := p.renderDecisionReport(inputs)
	if err != nil {
		return err
	}
	decisionPath := filepath.Join(p.outputDir, "DECISION_GATE_REPORT.md")
	if err := os.WriteFile(decisionPath, []byte(decisionMD), 0644); err != nil {
		return err
	}

	return nil
}

// convertToDataQuality converts SufficiencyResult to reporting.DataQualitySection.
func convertToDataQuality(result *SufficiencyResult) reporting.DataQualitySection {
	checks := make([]reporting.SufficiencyCheckRow, len(result.Checks))
	for i, c := range result.Checks {
		checks[i] = reporting.SufficiencyCheckRow{
			Name:      c.Name,
			Threshold: c.Threshold,
			Actual:    c.Actual,
			Pass:      c.Pass,
		}
	}
	return reporting.DataQualitySection{
		SufficiencyChecks: checks,
		IntegrityErrors:   result.Errors,
		AllChecksPassed:   result.AllPass,
	}
}

// writeInsufficientDataReport writes a decision report indicating insufficient data.
func (p *Phase1Pipeline) writeInsufficientDataReport(dataQuality reporting.DataQualitySection) error {
	var content string
	content += "# Phase 1 Decision Gate Report\n\n"
	content += "Generated at: " + p.clock().Format("2006-01-02 15:04:05 UTC") + "\n\n"
	content += "## Decision: INSUFFICIENT_DATA\n\n"
	content += "Data sufficiency checks failed. Cannot proceed with GO/NO-GO evaluation.\n\n"
	content += "### Failed Checks\n\n"
	content += "| Check | Threshold | Actual | Status |\n"
	content += "|-------|-----------|--------|--------|\n"
	for _, check := range dataQuality.SufficiencyChecks {
		status := "PASS"
		if !check.Pass {
			status = "FAIL"
		}
		content += "| " + check.Name + " | " + check.Threshold + " | " + check.Actual + " | " + status + " |\n"
	}
	content += "\n"

	if len(dataQuality.IntegrityErrors) > 0 {
		content += "### Integrity Errors\n\n"
		for _, err := range dataQuality.IntegrityErrors {
			content += "- " + err + "\n"
		}
		content += "\n"
	}

	content += "### Required Actions\n\n"
	content += "1. Collect more data until all sufficiency checks pass\n"
	content += "2. Fix any data integrity issues\n"
	content += "3. Re-run the pipeline\n"

	decisionPath := filepath.Join(p.outputDir, "DECISION_GATE_REPORT.md")
	return os.WriteFile(decisionPath, []byte(content), 0644)
}

// renderDecisionReport renders combined decision report for all strategies.
// Returns error on first validation failure (fail fast).
func (p *Phase1Pipeline) renderDecisionReport(inputs []*decision.DecisionInput) (string, error) {
	var content string
	content += "# Phase 1 Decision Gate Report\n\n"
	content += "Generated at: " + p.clock().Format("2006-01-02 15:04:05 UTC") + "\n\n"

	if len(inputs) == 0 {
		content += "No strategies to evaluate.\n"
		return content, nil
	}

	// Evaluate each input
	for i, input := range inputs {
		result, err := p.decisionEval.Evaluate(*input)
		if err != nil {
			// Fail fast on validation errors
			return "", err
		}

		if i > 0 {
			content += "---\n\n"
		}

		content += "## Strategy: " + input.StrategyID + " | " + input.EntryEventType + "\n\n"
		content += decision.RenderMarkdown(result)
		content += "\n"
	}

	return content, nil
}
