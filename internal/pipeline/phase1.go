package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/reporting"
	"solana-token-lab/internal/storage"
)

// Phase1Pipeline orchestrates report + decision generation.
type Phase1Pipeline struct {
	reportGen     *reporting.Generator
	decisionBuild *decision.Builder
	decisionEval  *decision.Evaluator
	outputDir     string
	clock         func() time.Time
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

// WithClock sets a custom clock function for deterministic output.
func (p *Phase1Pipeline) WithClock(clock func() time.Time) *Phase1Pipeline {
	p.clock = clock
	p.reportGen = p.reportGen.WithClock(clock)
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

	// Generate report
	report, err := p.reportGen.Generate(ctx)
	if err != nil {
		return err
	}

	// Write REPORT_PHASE1.md
	reportMD := reporting.RenderMarkdown(report)
	reportPath := filepath.Join(p.outputDir, "REPORT_PHASE1.md")
	if err := os.WriteFile(reportPath, []byte(reportMD), 0644); err != nil {
		return err
	}

	// Write STRATEGY_AGGREGATES.csv
	csvContent := reporting.RenderCSV(report.StrategyMetrics)
	csvPath := filepath.Join(p.outputDir, "STRATEGY_AGGREGATES.csv")
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		return err
	}

	// Build decision inputs and evaluate
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
	decisionMD := p.renderDecisionReport(inputs)
	decisionPath := filepath.Join(p.outputDir, "DECISION_GATE_REPORT.md")
	if err := os.WriteFile(decisionPath, []byte(decisionMD), 0644); err != nil {
		return err
	}

	return nil
}

// renderDecisionReport renders combined decision report for all strategies.
func (p *Phase1Pipeline) renderDecisionReport(inputs []*decision.DecisionInput) string {
	var content string
	content += "# Phase 1 Decision Gate Report\n\n"
	content += "Generated at: " + p.clock().Format("2006-01-02 15:04:05 UTC") + "\n\n"

	if len(inputs) == 0 {
		content += "No strategies to evaluate.\n"
		return content
	}

	// Evaluate each input
	for i, input := range inputs {
		result := p.decisionEval.Evaluate(*input)

		if i > 0 {
			content += "---\n\n"
		}

		content += "## Strategy: " + input.StrategyID + " | " + input.EntryEventType + "\n\n"
		content += decision.RenderMarkdown(result)
		content += "\n"
	}

	return content
}
