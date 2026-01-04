package decision

// Decision represents the final GO/NO-GO result.
type Decision string

const (
	DecisionGO   Decision = "GO"
	DecisionNOGO Decision = "NO-GO"
)

// DecisionInput contains numeric metrics for decision evaluation.
type DecisionInput struct {
	// Tokens with positive outcome percentage (realistic scenario)
	PositiveOutcomePct float64

	// Median outcome (realistic scenario)
	MedianOutcome float64

	// Outcome means for stability check
	RealisticMean float64
	DegradedMean  float64

	// Quantiles for outlier check (P25/P75 not available in source, not used in criteria)
	OutcomeP10 float64
	OutcomeP50 float64 // Same as MedianOutcome, used for outlier check
	OutcomeP90 float64

	// Strategy implementability (true if strategy exists and delay within scenario)
	StrategyImplementable bool

	// Strategy type + entry event type for context in report
	StrategyID     string
	EntryEventType string
	ScenarioID     string // should be realistic for decision
}

// CriterionResult represents pass/fail for one criterion.
type CriterionResult struct {
	Name      string
	Threshold string
	Actual    string
	Pass      bool
}

// DecisionResult contains the final decision with checklist.
type DecisionResult struct {
	Decision   Decision
	GOCriteria []CriterionResult // 5 GO criteria
	NOGOChecks []CriterionResult // 4 NO-GO triggers
}
