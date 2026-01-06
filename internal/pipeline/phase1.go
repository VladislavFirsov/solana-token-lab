package pipeline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/metrics"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/reporting"
	"solana-token-lab/internal/storage"
)

// Version constants for reproducibility
const (
	GeneratorVersion = "1.0.0"
	StrategyVersion  = "v1.0.0"
)

// Phase1Pipeline orchestrates report + decision generation.
type Phase1Pipeline struct {
	reportGen          *reporting.Generator
	decisionBuild      *decision.Builder
	decisionEval       *decision.Evaluator
	sufficiencyChecker *SufficiencyChecker
	aggregator         *metrics.Aggregator      // optional, for collecting missing candidate errors
	tradeStore         storage.TradeRecordStore // for CSV export
	outputDir          string
	clock              func() time.Time
	integrityErrors    []string // additional integrity errors (e.g., from aggregation)
	dataSource         string   // "fixtures" or "db" for replay command
	postgresDSN        string   // for DB mode replay command
	clickhouseDSN      string   // for DB mode replay command
	// Raw data stores for DataVersion hash (per REPORTING_SPEC)
	candidateStoreForHash    storage.CandidateStore
	priceTimeseriesStoreHash storage.PriceTimeseriesStore
	liqTimeseriesStoreHash   storage.LiquidityTimeseriesStore
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
		tradeStore:    tradeStore,
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

// WithDataSource sets the data source for reproducibility metadata.
// Use "fixtures" for fixture mode. For DB mode, use WithDBSource instead.
func (p *Phase1Pipeline) WithDataSource(source string) *Phase1Pipeline {
	p.dataSource = source
	return p
}

// WithDBSource sets the data source to DB mode with actual DSN values for replay command.
func (p *Phase1Pipeline) WithDBSource(postgresDSN, clickhouseDSN string) *Phase1Pipeline {
	p.dataSource = "db"
	p.postgresDSN = postgresDSN
	p.clickhouseDSN = clickhouseDSN
	return p
}

// WithRawDataStores sets raw data stores for DataVersion computation per REPORTING_SPEC.
// DataVersion = SHA256(SHA256(price_timeseries) || SHA256(liquidity_timeseries) || SHA256(candidates))
func (p *Phase1Pipeline) WithRawDataStores(
	candidateStore storage.CandidateStore,
	priceStore storage.PriceTimeseriesStore,
	liqStore storage.LiquidityTimeseriesStore,
) *Phase1Pipeline {
	p.candidateStoreForHash = candidateStore
	p.priceTimeseriesStoreHash = priceStore
	p.liqTimeseriesStoreHash = liqStore
	return p
}

// Run executes full pipeline and writes output files:
// - REPORT_PHASE1.md
// - strategy_aggregates.csv
// - trade_records.csv
// - scenario_outcomes.csv
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

	// 3. Load trades early (needed for DataVersion hash and CSV export)
	trades, err := p.tradeStore.GetAll(ctx)
	if err != nil {
		return err
	}

	// 4. Populate Executive Summary
	p.populateExecutiveSummary(report)

	// 5. Populate Reproducibility metadata (needs trades for DataVersion)
	p.populateReproducibility(ctx, report, trades)

	// 6. Set decision checklist reference
	report.DecisionChecklistRef = "docs/DECISION_CHECKLIST.md"

	// 7. Write REPORT_PHASE1.md
	reportMD := reporting.RenderMarkdown(report)
	reportPath := filepath.Join(p.outputDir, "REPORT_PHASE1.md")
	if err := os.WriteFile(reportPath, []byte(reportMD), 0644); err != nil {
		return err
	}

	// 8. Write strategy_aggregates.csv (18 columns per REPORTING_SPEC)
	aggCSV := reporting.RenderStrategyAggregatesCSV(report.StrategyMetrics)
	aggPath := filepath.Join(p.outputDir, "strategy_aggregates.csv")
	if err := os.WriteFile(aggPath, []byte(aggCSV), 0644); err != nil {
		return err
	}

	// 9. Write trade_records.csv (27 columns per REPORTING_SPEC)
	tradeCSV := reporting.RenderTradeRecordsCSV(trades)
	tradePath := filepath.Join(p.outputDir, "trade_records.csv")
	if err := os.WriteFile(tradePath, []byte(tradeCSV), 0644); err != nil {
		return err
	}

	// 9. Write scenario_outcomes.csv (6 columns per REPORTING_SPEC)
	scenarioCSV := reporting.RenderScenarioOutcomesCSV(report.ScenarioSensitivity)
	scenarioPath := filepath.Join(p.outputDir, "scenario_outcomes.csv")
	if err := os.WriteFile(scenarioPath, []byte(scenarioCSV), 0644); err != nil {
		return err
	}

	// 10. If sufficiency fails -> INSUFFICIENT_DATA decision
	if p.sufficiencyChecker != nil && !dataQuality.AllChecksPassed {
		report.ExecutiveSummary.Decision = string(decision.DecisionInsufficientData)

		// Re-render REPORT_PHASE1.md with updated decision
		reportMD = reporting.RenderMarkdown(report)
		if err := os.WriteFile(reportPath, []byte(reportMD), 0644); err != nil {
			return err
		}

		if err := p.writeInsufficientDataReport(dataQuality); err != nil {
			return err
		}

		// Write additional artifacts even for INSUFFICIENT_DATA
		if err := p.writeReportJSON(report); err != nil {
			return err
		}
		if err := p.writeMetadata(report); err != nil {
			return err
		}
		if err := p.writeMetricsQueries(); err != nil {
			return err
		}
		if err := p.writeChecksums(); err != nil {
			return err
		}

		return nil
	}

	// 11. Otherwise proceed with GO/NO-GO evaluation
	inputs, err := p.decisionBuild.BuildAll(report)
	if err != nil {
		// If no realistic scenarios or missing pessimistic scenario, treat as insufficient data
		if err == decision.ErrNoRealisticScenario || err == decision.ErrMissingPessimisticScenario {
			report.ExecutiveSummary.Decision = string(decision.DecisionInsufficientData)

			// Re-render REPORT_PHASE1.md with updated decision
			reportMD = reporting.RenderMarkdown(report)
			if err := os.WriteFile(reportPath, []byte(reportMD), 0644); err != nil {
				return err
			}

			// Write insufficient data decision report
			dataQuality.IntegrityErrors = append(dataQuality.IntegrityErrors, err.Error())
			dataQuality.AllChecksPassed = false
			return p.writeInsufficientDataReport(dataQuality)
		}
		return err
	}

	// Evaluate each strategy and render combined decision report
	decisionMD, overallDecision, err := p.renderDecisionReportWithDecision(inputs)
	if err != nil {
		return err
	}

	// Update executive summary with overall decision
	report.ExecutiveSummary.Decision = string(overallDecision)

	// Re-render report with updated decision
	reportMD = reporting.RenderMarkdown(report)
	if err := os.WriteFile(reportPath, []byte(reportMD), 0644); err != nil {
		return err
	}

	decisionPath := filepath.Join(p.outputDir, "DECISION_GATE_REPORT.md")
	if err := os.WriteFile(decisionPath, []byte(decisionMD), 0644); err != nil {
		return err
	}

	// Write additional artifacts per REPORTING_SPEC
	if err := p.writeReportJSON(report); err != nil {
		return err
	}
	if err := p.writeMetadata(report); err != nil {
		return err
	}
	if err := p.writeMetricsQueries(); err != nil {
		return err
	}
	// writeChecksums must be last as it computes hashes of all other files
	if err := p.writeChecksums(); err != nil {
		return err
	}

	return nil
}

// populateExecutiveSummary fills in executive summary from report data.
func (p *Phase1Pipeline) populateExecutiveSummary(report *reporting.Report) {
	summary := &report.ExecutiveSummary

	// Token counts
	summary.NewTokenCount = report.DataSummary.NewTokenCandidates
	summary.ActiveTokenCount = report.DataSummary.ActiveTokenCandidates

	// Data period
	if report.DataSummary.DateRangeStart > 0 {
		summary.DataPeriodStart = time.UnixMilli(report.DataSummary.DateRangeStart).UTC()
	}
	if report.DataSummary.DateRangeEnd > 0 {
		summary.DataPeriodEnd = time.UnixMilli(report.DataSummary.DateRangeEnd).UTC()
	}

	// Find best strategy under realistic scenario (highest median)
	var bestMetric *reporting.StrategyMetricRow
	var pessimisticMedian float64
	for i := range report.StrategyMetrics {
		m := &report.StrategyMetrics[i]
		if m.ScenarioID == domain.ScenarioRealistic {
			if bestMetric == nil || m.OutcomeMedian > bestMetric.OutcomeMedian {
				bestMetric = m
			}
		}
	}

	if bestMetric != nil {
		summary.BestStrategy = bestMetric.StrategyID
		summary.BestEntryType = bestMetric.EntryEventType
		summary.WinRateRealistic = bestMetric.WinRate
		summary.MedianRealistic = bestMetric.OutcomeMedian

		// Find corresponding pessimistic median
		for _, m := range report.StrategyMetrics {
			if m.StrategyID == bestMetric.StrategyID &&
				m.EntryEventType == bestMetric.EntryEventType &&
				m.ScenarioID == domain.ScenarioPessimistic {
				pessimisticMedian = m.OutcomeMedian
				break
			}
		}
		summary.MedianPessimistic = pessimisticMedian
	}

	// Decision will be set after evaluation
	summary.Decision = "PENDING"
}

// populateReproducibility fills in reproducibility metadata.
func (p *Phase1Pipeline) populateReproducibility(ctx context.Context, report *reporting.Report, trades []*domain.TradeRecord) {
	report.Reproducibility = reporting.ReproducibilityMetadata{
		ReportTimestamp:  p.clock(),
		GeneratorVersion: GeneratorVersion,
		DataVersion:      p.computeDataVersion(ctx, report, trades),
		StrategyVersion:  StrategyVersion,
		ReplayCommitHash: getGitCommitHash(),
		ReplayCommand:    p.buildReplayCommand(),
	}
}

// buildReplayCommand returns the command to reproduce this report.
func (p *Phase1Pipeline) buildReplayCommand() string {
	switch p.dataSource {
	case "fixtures":
		return "go run cmd/report/main.go --use-fixtures"
	case "db":
		// Use actual DSN flags for reproducibility
		return fmt.Sprintf("go run cmd/report/main.go --postgres-dsn %q --clickhouse-dsn %q",
			p.postgresDSN, p.clickhouseDSN)
	default:
		// Default to fixtures if not specified
		return "go run cmd/report/main.go --use-fixtures"
	}
}

// computeDataVersion computes SHA256 hash per REPORTING_SPEC section 3.2:
// data_version = SHA256(SHA256(price_timeseries) || SHA256(liquidity_timeseries) || SHA256(candidates))
// Falls back to trades-based hash if raw data stores not configured or on error.
func (p *Phase1Pipeline) computeDataVersion(ctx context.Context, report *reporting.Report, trades []*domain.TradeRecord) string {
	// Use raw data stores if configured (per spec)
	if p.candidateStoreForHash != nil && p.priceTimeseriesStoreHash != nil && p.liqTimeseriesStoreHash != nil {
		hash, err := p.computeDataVersionFromRaw(ctx)
		if err != nil {
			// Fall back to trades-based hash on error
			fmt.Fprintf(os.Stderr, "WARNING: raw data hash failed, using fallback: %v\n", err)
			return p.computeDataVersionFromTrades(report, trades)
		}
		return hash
	}
	// Fallback: use trades-based hash
	return p.computeDataVersionFromTrades(report, trades)
}

// computeDataVersionFromRaw computes hash from raw data per REPORTING_SPEC.
// Returns error if any hash computation fails to ensure deterministic results.
func (p *Phase1Pipeline) computeDataVersionFromRaw(ctx context.Context) (string, error) {
	// Hash candidates
	candidatesHash, err := p.hashCandidates(ctx)
	if err != nil {
		return "", fmt.Errorf("hash candidates: %w", err)
	}

	// Hash price timeseries
	priceHash, err := p.hashPriceTimeseries(ctx)
	if err != nil {
		return "", fmt.Errorf("hash price timeseries: %w", err)
	}

	// Hash liquidity timeseries
	liqHash, err := p.hashLiquidityTimeseries(ctx)
	if err != nil {
		return "", fmt.Errorf("hash liquidity timeseries: %w", err)
	}

	// Combine: SHA256(priceHash || liqHash || candidatesHash)
	h := sha256.New()
	h.Write(priceHash)
	h.Write(liqHash)
	h.Write(candidatesHash)

	// Full SHA256 hash per REPORTING_SPEC.md section 3.1
	return hex.EncodeToString(h.Sum(nil)), nil
}

// hashCandidates returns SHA256 of all candidates (sorted by candidate_id).
// Per DISCOVERY_SPEC.md: includes pool in hash as it's part of candidate_id discriminator.
// Returns error if database queries fail.
func (p *Phase1Pipeline) hashCandidates(ctx context.Context) ([]byte, error) {
	h := sha256.New()

	// Get all candidates with error handling
	newTokens, err := p.candidateStoreForHash.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		return h.Sum(nil), fmt.Errorf("get NEW_TOKEN candidates: %w", err)
	}
	activeTokens, err := p.candidateStoreForHash.GetBySource(ctx, domain.SourceActiveToken)
	if err != nil {
		return h.Sum(nil), fmt.Errorf("get ACTIVE_TOKEN candidates: %w", err)
	}
	// Safe merge: create new slice to avoid mutating original slices
	all := make([]*domain.TokenCandidate, 0, len(newTokens)+len(activeTokens))
	all = append(all, newTokens...)
	all = append(all, activeTokens...)

	// Sort by candidate_id for determinism
	sort.Slice(all, func(i, j int) bool {
		return all[i].CandidateID < all[j].CandidateID
	})

	// Hash each candidate (including pool per DISCOVERY_SPEC.md)
	for _, c := range all {
		pool := ""
		if c.Pool != nil {
			pool = *c.Pool
		}
		line := fmt.Sprintf("%s|%s|%s|%s|%s|%d|%d|%d\n",
			c.CandidateID, c.Source, c.Mint, pool, c.TxSignature,
			c.EventIndex, c.Slot, c.DiscoveredAt)
		h.Write([]byte(line))
	}

	return h.Sum(nil), nil
}

// hashPriceTimeseries returns SHA256 of all price timeseries points.
// Uses GetGlobalTimeRange + GetByTimeRange for efficiency.
// Returns error if database queries fail.
func (p *Phase1Pipeline) hashPriceTimeseries(ctx context.Context) ([]byte, error) {
	h := sha256.New()
	var firstErr error

	// Get all candidates to iterate their price data
	newTokens, err := p.candidateStoreForHash.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		return h.Sum(nil), fmt.Errorf("get NEW_TOKEN candidates: %w", err)
	}
	activeTokens, err := p.candidateStoreForHash.GetBySource(ctx, domain.SourceActiveToken)
	if err != nil {
		return h.Sum(nil), fmt.Errorf("get ACTIVE_TOKEN candidates: %w", err)
	}
	// Safe merge: create new slice to avoid mutating original slices
	all := make([]*domain.TokenCandidate, 0, len(newTokens)+len(activeTokens))
	all = append(all, newTokens...)
	all = append(all, activeTokens...)

	// Sort candidates for determinism
	sort.Slice(all, func(i, j int) bool {
		return all[i].CandidateID < all[j].CandidateID
	})

	// Hash price points for each candidate
	for _, c := range all {
		points, err := p.priceTimeseriesStoreHash.GetByCandidateID(ctx, c.CandidateID)
		if err != nil {
			// Capture first error but continue to hash remaining candidates
			if firstErr == nil {
				firstErr = fmt.Errorf("get price timeseries for %s: %w", c.CandidateID, err)
			}
			continue
		}
		for _, pt := range points {
			line := fmt.Sprintf("%s|%d|%d|%.8f|%.8f|%d\n",
				pt.CandidateID, pt.TimestampMs, pt.Slot,
				pt.Price, pt.Volume, pt.SwapCount)
			h.Write([]byte(line))
		}
	}

	return h.Sum(nil), firstErr
}

// hashLiquidityTimeseries returns SHA256 of all liquidity timeseries points.
// Returns error if database queries fail.
func (p *Phase1Pipeline) hashLiquidityTimeseries(ctx context.Context) ([]byte, error) {
	h := sha256.New()
	var firstErr error

	// Get all candidates to iterate their liquidity data
	newTokens, err := p.candidateStoreForHash.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		return h.Sum(nil), fmt.Errorf("get NEW_TOKEN candidates: %w", err)
	}
	activeTokens, err := p.candidateStoreForHash.GetBySource(ctx, domain.SourceActiveToken)
	if err != nil {
		return h.Sum(nil), fmt.Errorf("get ACTIVE_TOKEN candidates: %w", err)
	}
	// Safe merge: create new slice to avoid mutating original slices
	all := make([]*domain.TokenCandidate, 0, len(newTokens)+len(activeTokens))
	all = append(all, newTokens...)
	all = append(all, activeTokens...)

	// Sort candidates for determinism
	sort.Slice(all, func(i, j int) bool {
		return all[i].CandidateID < all[j].CandidateID
	})

	// Hash liquidity points for each candidate
	for _, c := range all {
		points, err := p.liqTimeseriesStoreHash.GetByCandidateID(ctx, c.CandidateID)
		if err != nil {
			// Capture first error but continue to hash remaining candidates
			if firstErr == nil {
				firstErr = fmt.Errorf("get liquidity timeseries for %s: %w", c.CandidateID, err)
			}
			continue
		}
		for _, pt := range points {
			line := fmt.Sprintf("%s|%d|%d|%.8f|%.8f|%.8f\n",
				pt.CandidateID, pt.TimestampMs, pt.Slot,
				pt.Liquidity, pt.LiquidityToken, pt.LiquidityQuote)
			h.Write([]byte(line))
		}
	}

	return h.Sum(nil), firstErr
}

// computeDataVersionFromTrades is fallback when raw stores not configured.
func (p *Phase1Pipeline) computeDataVersionFromTrades(report *reporting.Report, trades []*domain.TradeRecord) string {
	h := sha256.New()

	// Part 1: Strategy metrics (aggregated data)
	var metricParts []string
	for _, m := range report.StrategyMetrics {
		part := fmt.Sprintf("%s|%s|%s|%d|%.6f|%.6f|%.6f|%.6f",
			m.StrategyID, m.ScenarioID, m.EntryEventType,
			m.TotalTrades, m.WinRate, m.OutcomeMedian, m.OutcomeP25, m.OutcomeP75)
		metricParts = append(metricParts, part)
	}
	sort.Strings(metricParts)
	h.Write([]byte("METRICS\n"))
	h.Write([]byte(strings.Join(metricParts, "\n")))

	// Part 2: Individual trade records (trade_id + outcome)
	var tradeParts []string
	for _, t := range trades {
		part := fmt.Sprintf("%s|%.6f", t.TradeID, t.Outcome)
		tradeParts = append(tradeParts, part)
	}
	sort.Strings(tradeParts)
	h.Write([]byte("\nTRADES\n"))
	h.Write([]byte(strings.Join(tradeParts, "\n")))

	// Full SHA256 hash per REPORTING_SPEC.md section 3.1
	return hex.EncodeToString(h.Sum(nil))
}

// getGitCommitHash returns current git commit hash or "unknown" if not in git repo.
func getGitCommitHash() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out.String())
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
// Deprecated: use renderDecisionReportWithDecision instead.
func (p *Phase1Pipeline) renderDecisionReport(inputs []*decision.DecisionInput) (string, error) {
	md, _, err := p.renderDecisionReportWithDecision(inputs)
	return md, err
}

// renderDecisionReportWithDecision renders combined decision report and returns overall decision.
// Overall decision is based on best strategy (highest RealisticMedian):
// GO only if the best strategy is GO, otherwise NO-GO.
func (p *Phase1Pipeline) renderDecisionReportWithDecision(inputs []*decision.DecisionInput) (string, decision.Decision, error) {
	var content string
	content += "# Phase 1 Decision Gate Report\n\n"
	content += "Generated at: " + p.clock().Format("2006-01-02 15:04:05 UTC") + "\n\n"

	if len(inputs) == 0 {
		content += "No strategies to evaluate.\n"
		return content, decision.DecisionNOGO, nil
	}

	// Find best strategy by RealisticMedian
	bestIdx := 0
	bestMedian := inputs[0].RealisticMedian
	for i, input := range inputs {
		if input.RealisticMedian > bestMedian {
			bestMedian = input.RealisticMedian
			bestIdx = i
		}
	}

	// Evaluate all strategies and collect results
	results := make([]*decision.DecisionResult, len(inputs))
	for i, input := range inputs {
		result, err := p.decisionEval.Evaluate(*input)
		if err != nil {
			// Fail fast on validation errors
			return "", decision.DecisionNOGO, err
		}
		results[i] = result
	}

	// Overall decision = decision of best strategy
	overallDecision := results[bestIdx].Decision

	// Render each strategy section
	for i, input := range inputs {
		if i > 0 {
			content += "---\n\n"
		}

		isBest := i == bestIdx
		strategyHeader := "## Strategy: " + input.StrategyID + " | " + input.EntryEventType
		if isBest {
			strategyHeader += " ⭐ (Best)"
		}
		content += strategyHeader + "\n\n"
		content += decision.RenderMarkdown(results[i])
		content += "\n"
	}

	// Add overall summary
	content += "---\n\n"
	content += "## Overall Decision\n\n"
	content += fmt.Sprintf("**%s** (based on best strategy: %s | %s, median=%.4f)\n",
		string(overallDecision),
		inputs[bestIdx].StrategyID,
		inputs[bestIdx].EntryEventType,
		bestMedian)

	return content, overallDecision, nil
}

// writeReportJSON writes report.json with full machine-readable report per REPORTING_SPEC.
func (p *Phase1Pipeline) writeReportJSON(report *reporting.Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	path := filepath.Join(p.outputDir, "report.json")
	return os.WriteFile(path, data, 0644)
}

// writeMetadata writes metadata.json with report metadata per REPORTING_SPEC.
func (p *Phase1Pipeline) writeMetadata(report *reporting.Report) error {
	metadata := map[string]interface{}{
		"report_timestamp":   report.Reproducibility.ReportTimestamp.Format(time.RFC3339),
		"generator_version":  report.Reproducibility.GeneratorVersion,
		"data_version":       report.Reproducibility.DataVersion,
		"strategy_version":   report.Reproducibility.StrategyVersion,
		"replay_commit_hash": report.Reproducibility.ReplayCommitHash,
		"replay_command":     report.Reproducibility.ReplayCommand,
		"strategy_count":     report.StrategyCount,
		"scenario_count":     report.ScenarioCount,
		"decision":           report.ExecutiveSummary.Decision,
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	path := filepath.Join(p.outputDir, "metadata.json")
	return os.WriteFile(path, data, 0644)
}

// writeChecksums writes checksums.sha256 for all output files per REPORTING_SPEC.
func (p *Phase1Pipeline) writeChecksums() error {
	files := []string{
		"REPORT_PHASE1.md",
		"DECISION_GATE_REPORT.md",
		"report.json",
		"strategy_aggregates.csv",
		"trade_records.csv",
		"scenario_outcomes.csv",
		"metadata.json",
		"metrics_queries.sql",
	}

	var checksums []string
	for _, file := range files {
		path := filepath.Join(p.outputDir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			// Skip files that don't exist (e.g., if INSUFFICIENT_DATA decision)
			continue
		}
		hash := sha256.Sum256(data)
		checksums = append(checksums, fmt.Sprintf("%s  %s", hex.EncodeToString(hash[:]), file))
	}

	content := strings.Join(checksums, "\n") + "\n"
	path := filepath.Join(p.outputDir, "checksums.sha256")
	return os.WriteFile(path, []byte(content), 0644)
}

// writeMetricsQueries writes metrics_queries.sql with SQL templates per REPORTING_SPEC.
func (p *Phase1Pipeline) writeMetricsQueries() error {
	queries := `-- Metrics Queries for Phase 1 Report
-- Generated by Phase1Pipeline

-- Strategy aggregates by scenario
SELECT
    strategy_id,
    scenario_id,
    entry_event_type,
    total_trades,
    win_rate,
    outcome_median,
    outcome_p25,
    outcome_p75
FROM strategy_aggregates
ORDER BY strategy_id, scenario_id, entry_event_type;

-- Top performing strategies (Realistic scenario)
SELECT
    strategy_id,
    entry_event_type,
    win_rate,
    outcome_median,
    max_drawdown
FROM strategy_aggregates
WHERE scenario_id = 'Realistic'
ORDER BY outcome_median DESC
LIMIT 10;

-- Scenario sensitivity comparison
SELECT
    a.strategy_id,
    a.entry_event_type,
    a.outcome_median AS realistic_median,
    b.outcome_median AS pessimistic_median,
    (a.outcome_median - b.outcome_median) / a.outcome_median * 100 AS degradation_pct
FROM strategy_aggregates a
JOIN strategy_aggregates b ON a.strategy_id = b.strategy_id
    AND a.entry_event_type = b.entry_event_type
WHERE a.scenario_id = 'Realistic' AND b.scenario_id = 'Pessimistic';
`
	path := filepath.Join(p.outputDir, "metrics_queries.sql")
	return os.WriteFile(path, []byte(queries), 0644)
}
