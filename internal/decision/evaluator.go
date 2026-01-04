package decision

import "fmt"

// Evaluator evaluates decision criteria.
type Evaluator struct{}

// NewEvaluator creates a new decision evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate produces DecisionResult from DecisionInput.
// GO if ALL criteria pass and NO NO-GO triggers.
// NO-GO if ANY criterion fails or ANY trigger fires.
func (e *Evaluator) Evaluate(input DecisionInput) *DecisionResult {
	goCriteria := e.evaluateGOCriteria(input)
	nogoChecks := e.evaluateNOGOTriggers(input)

	// Count passes and triggers
	allGOPass := true
	for _, c := range goCriteria {
		if !c.Pass {
			allGOPass = false
			break
		}
	}

	anyNOGOTriggered := false
	for _, c := range nogoChecks {
		if !c.Pass { // Pass=false means triggered
			anyNOGOTriggered = true
			break
		}
	}

	decision := DecisionGO
	if !allGOPass || anyNOGOTriggered {
		decision = DecisionNOGO
	}

	return &DecisionResult{
		Decision:   decision,
		GOCriteria: goCriteria,
		NOGOChecks: nogoChecks,
	}
}

// evaluateGOCriteria evaluates the 5 GO criteria.
func (e *Evaluator) evaluateGOCriteria(input DecisionInput) []CriterionResult {
	criteria := make([]CriterionResult, 5)

	// 1. PositiveOutcomePct >= 5%
	criteria[0] = CriterionResult{
		Name:      "Positive outcome tokens",
		Threshold: ">= 5%",
		Actual:    fmt.Sprintf("%.2f%%", input.PositiveOutcomePct),
		Pass:      input.PositiveOutcomePct >= 5.0,
	}

	// 2. MedianOutcome > 0
	criteria[1] = CriterionResult{
		Name:      "Median outcome",
		Threshold: "> 0",
		Actual:    fmt.Sprintf("%.4f", input.MedianOutcome),
		Pass:      input.MedianOutcome > 0,
	}

	// 3. Stable under degradation: DegradedMean > 0 AND DegradedMean/RealisticMean >= 0.5
	stabilityPass := false
	var stabilityActual string
	if input.RealisticMean > 0 {
		ratio := input.DegradedMean / input.RealisticMean
		stabilityPass = input.DegradedMean > 0 && ratio >= 0.5
		stabilityActual = fmt.Sprintf("DegradedMean=%.4f, Ratio=%.2f", input.DegradedMean, ratio)
	} else {
		stabilityActual = fmt.Sprintf("DegradedMean=%.4f, RealisticMean=%.4f", input.DegradedMean, input.RealisticMean)
	}
	criteria[2] = CriterionResult{
		Name:      "Stable under degradation",
		Threshold: "DegradedMean > 0 AND ratio >= 0.5",
		Actual:    stabilityActual,
		Pass:      stabilityPass,
	}

	// 4. Not dominated by outliers: P50 > 0 (same as median check)
	criteria[3] = CriterionResult{
		Name:      "Not dominated by outliers",
		Threshold: "P50 > 0",
		Actual:    fmt.Sprintf("%.4f", input.OutcomeP50),
		Pass:      input.OutcomeP50 > 0,
	}

	// 5. Entry/exit implementable: StrategyImplementable == true
	criteria[4] = CriterionResult{
		Name:      "Entry/exit implementable",
		Threshold: "true",
		Actual:    fmt.Sprintf("%t", input.StrategyImplementable),
		Pass:      input.StrategyImplementable,
	}

	return criteria
}

// evaluateNOGOTriggers evaluates the 4 NO-GO triggers.
// Pass=true means NOT triggered, Pass=false means triggered.
func (e *Evaluator) evaluateNOGOTriggers(input DecisionInput) []CriterionResult {
	checks := make([]CriterionResult, 4)

	// 1. PositiveOutcomePct < 5% triggers NO-GO
	triggered1 := input.PositiveOutcomePct < 5.0
	checks[0] = CriterionResult{
		Name:      "Low positive outcome",
		Threshold: "< 5%",
		Actual:    fmt.Sprintf("%.2f%%", input.PositiveOutcomePct),
		Pass:      !triggered1, // Pass means NOT triggered
	}

	// 2. MedianOutcome <= 0 triggers NO-GO
	triggered2 := input.MedianOutcome <= 0
	checks[1] = CriterionResult{
		Name:      "Negative/zero median",
		Threshold: "<= 0",
		Actual:    fmt.Sprintf("%.4f", input.MedianOutcome),
		Pass:      !triggered2,
	}

	// 3. Edge disappears: RealisticMean > 0 && DegradedMean <= 0 triggers NO-GO
	triggered3 := input.RealisticMean > 0 && input.DegradedMean <= 0
	checks[2] = CriterionResult{
		Name:      "Edge disappears under degradation",
		Threshold: "RealisticMean > 0 AND DegradedMean <= 0",
		Actual:    fmt.Sprintf("RealisticMean=%.4f, DegradedMean=%.4f", input.RealisticMean, input.DegradedMean),
		Pass:      !triggered3,
	}

	// 4. Entry not implementable: StrategyImplementable == false triggers NO-GO
	triggered4 := !input.StrategyImplementable
	checks[3] = CriterionResult{
		Name:      "Entry not implementable",
		Threshold: "StrategyImplementable == false",
		Actual:    fmt.Sprintf("%t", input.StrategyImplementable),
		Pass:      !triggered4,
	}

	return checks
}
