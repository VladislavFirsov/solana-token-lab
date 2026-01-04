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
// Returns error if input validation fails.
func (e *Evaluator) Evaluate(input DecisionInput) (*DecisionResult, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

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
	}, nil
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

	// 3. Stable under pessimistic scenario: PessimisticMedian > 0 AND PessimisticMedian/RealisticMedian >= 0.5
	// Per DECISION_GATE.md: sensitivity analysis uses Realistic vs Pessimistic (not Degraded)
	stabilityPass := false
	var stabilityActual string
	if input.RealisticMedian > 0 {
		ratio := input.PessimisticMedian / input.RealisticMedian
		stabilityPass = input.PessimisticMedian > 0 && ratio >= 0.5
		stabilityActual = fmt.Sprintf("PessimisticMedian=%.4f, Ratio=%.2f", input.PessimisticMedian, ratio)
	} else {
		stabilityActual = fmt.Sprintf("PessimisticMedian=%.4f, RealisticMedian=%.4f", input.PessimisticMedian, input.RealisticMedian)
	}
	criteria[2] = CriterionResult{
		Name:      "Stable under pessimistic scenario",
		Threshold: "PessimisticMedian > 0 AND ratio >= 0.5",
		Actual:    stabilityActual,
		Pass:      stabilityPass,
	}

	// 4. Not dominated by outliers: check that P25 > 0 OR (P75 - P25) / median < 3.0
	// This verifies the distribution isn't driven by extreme tail values
	outlierPass := false
	var outlierActual string
	if input.OutcomeP50 != 0 {
		iqrRatio := (input.OutcomeP75 - input.OutcomeP25) / input.OutcomeP50
		outlierPass = input.OutcomeP25 > 0 || iqrRatio < 3.0
		outlierActual = fmt.Sprintf("P25=%.4f, IQR/median=%.2f", input.OutcomeP25, iqrRatio)
	} else {
		outlierPass = input.OutcomeP25 > 0
		outlierActual = fmt.Sprintf("P25=%.4f, P50=0 (undefined ratio)", input.OutcomeP25)
	}
	criteria[3] = CriterionResult{
		Name:      "Not dominated by outliers",
		Threshold: "P25 > 0 OR IQR/median < 3.0",
		Actual:    outlierActual,
		Pass:      outlierPass,
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

	// 3. Edge disappears: RealisticMedian > 0 && PessimisticMedian <= 0 triggers NO-GO
	// Per DECISION_GATE.md: sensitivity uses Realistic vs Pessimistic
	triggered3 := input.RealisticMedian > 0 && input.PessimisticMedian <= 0
	checks[2] = CriterionResult{
		Name:      "Edge disappears under pessimistic scenario",
		Threshold: "RealisticMedian > 0 AND PessimisticMedian <= 0",
		Actual:    fmt.Sprintf("RealisticMedian=%.4f, PessimisticMedian=%.4f", input.RealisticMedian, input.PessimisticMedian),
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
