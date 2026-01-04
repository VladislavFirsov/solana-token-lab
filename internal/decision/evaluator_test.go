package decision

import (
	"strings"
	"testing"

	"solana-token-lab/internal/domain"
)

func TestEvaluate_GO(t *testing.T) {
	evaluator := NewEvaluator()

	// All criteria pass, no triggers
	// Per DECISION_GATE.md: stability uses Pessimistic vs Realistic
	input := DecisionInput{
		PositiveOutcomePct:    10.0, // >= 5%
		MedianOutcome:         0.05, // > 0
		RealisticMean:         0.08,
		RealisticMedian:       0.05,
		PessimisticMean:       0.04,
		PessimisticMedian:     0.03, // > 0 and ratio = 0.6 >= 0.5
		OutcomeP10:            -0.02,
		OutcomeP25:            0.02,
		OutcomeP50:            0.05, // > 0
		OutcomeP75:            0.10,
		OutcomeP90:            0.15,
		StrategyImplementable: true,
		StrategyID:            "TIME_EXIT",
		EntryEventType:        "NEW_TOKEN",
		ScenarioID:            domain.ScenarioRealistic,
	}

	result, err := evaluator.Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionGO {
		t.Errorf("Expected GO, got %s", result.Decision)
	}

	// All 5 GO criteria should pass
	for i, c := range result.GOCriteria {
		if !c.Pass {
			t.Errorf("GO criterion %d (%s) should pass, got fail", i+1, c.Name)
		}
	}

	// All 4 NO-GO triggers should NOT be triggered (Pass=true)
	for i, c := range result.NOGOChecks {
		if !c.Pass {
			t.Errorf("NO-GO trigger %d (%s) should not be triggered", i+1, c.Name)
		}
	}
}

func TestEvaluate_NOGO_LowPositiveOutcome(t *testing.T) {
	evaluator := NewEvaluator()

	input := DecisionInput{
		PositiveOutcomePct:    3.0, // < 5% - triggers NO-GO
		MedianOutcome:         0.05,
		RealisticMean:         0.08,
		RealisticMedian:       0.05,
		PessimisticMean:       0.04,
		PessimisticMedian:     0.03,
		OutcomeP25:            0.02,
		OutcomeP50:            0.05,
		OutcomeP75:            0.10,
		StrategyImplementable: true,
		StrategyID:            "TEST",
		ScenarioID:            domain.ScenarioRealistic,
	}

	result, err := evaluator.Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionNOGO {
		t.Errorf("Expected NO-GO, got %s", result.Decision)
	}

	// First GO criterion (positive outcome) should fail
	if result.GOCriteria[0].Pass {
		t.Error("GO criterion 1 (positive outcome) should fail")
	}

	// First NO-GO trigger should be triggered (Pass=false)
	if result.NOGOChecks[0].Pass {
		t.Error("NO-GO trigger 1 (low positive outcome) should be triggered")
	}
}

func TestEvaluate_NOGO_NegativeMedian(t *testing.T) {
	evaluator := NewEvaluator()

	input := DecisionInput{
		PositiveOutcomePct:    10.0,
		MedianOutcome:         -0.02, // <= 0 - triggers NO-GO
		RealisticMean:         0.01,
		RealisticMedian:       -0.02,
		PessimisticMean:       0.005,
		PessimisticMedian:     0.002,
		OutcomeP25:            -0.05,
		OutcomeP50:            -0.02,
		OutcomeP75:            0.05,
		StrategyImplementable: true,
		StrategyID:            "TEST",
		ScenarioID:            domain.ScenarioRealistic,
	}

	result, err := evaluator.Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionNOGO {
		t.Errorf("Expected NO-GO, got %s", result.Decision)
	}

	// Second GO criterion (median outcome) should fail
	if result.GOCriteria[1].Pass {
		t.Error("GO criterion 2 (median outcome) should fail")
	}

	// Second NO-GO trigger should be triggered
	if result.NOGOChecks[1].Pass {
		t.Error("NO-GO trigger 2 (negative median) should be triggered")
	}
}

func TestEvaluate_NOGO_EdgeDisappears(t *testing.T) {
	evaluator := NewEvaluator()

	// Per DECISION_GATE.md: edge disappears = Realistic > 0 but Pessimistic <= 0
	input := DecisionInput{
		PositiveOutcomePct:    10.0,
		MedianOutcome:         0.05,
		RealisticMean:         0.08,
		RealisticMedian:       0.05, // > 0
		PessimisticMean:       -0.01,
		PessimisticMedian:     -0.02, // <= 0 - edge disappears
		OutcomeP25:            0.02,
		OutcomeP50:            0.05,
		OutcomeP75:            0.10,
		StrategyImplementable: true,
		StrategyID:            "TEST",
		ScenarioID:            domain.ScenarioRealistic,
	}

	result, err := evaluator.Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionNOGO {
		t.Errorf("Expected NO-GO, got %s", result.Decision)
	}

	// Third GO criterion (stability) should fail
	if result.GOCriteria[2].Pass {
		t.Error("GO criterion 3 (stability) should fail")
	}

	// Third NO-GO trigger should be triggered
	if result.NOGOChecks[2].Pass {
		t.Error("NO-GO trigger 3 (edge disappears) should be triggered")
	}
}

func TestEvaluate_NOGO_NotImplementable(t *testing.T) {
	evaluator := NewEvaluator()

	input := DecisionInput{
		PositiveOutcomePct:    10.0,
		MedianOutcome:         0.05,
		RealisticMean:         0.08,
		RealisticMedian:       0.05,
		PessimisticMean:       0.04,
		PessimisticMedian:     0.03,
		OutcomeP25:            0.02,
		OutcomeP50:            0.05,
		OutcomeP75:            0.10,
		StrategyImplementable: false, // not implementable - triggers NO-GO
		StrategyID:            "TEST",
		ScenarioID:            domain.ScenarioRealistic,
	}

	result, err := evaluator.Evaluate(input)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Decision != DecisionNOGO {
		t.Errorf("Expected NO-GO, got %s", result.Decision)
	}

	// Fifth GO criterion (implementable) should fail
	if result.GOCriteria[4].Pass {
		t.Error("GO criterion 5 (implementable) should fail")
	}

	// Fourth NO-GO trigger should be triggered
	if result.NOGOChecks[3].Pass {
		t.Error("NO-GO trigger 4 (not implementable) should be triggered")
	}
}

func TestEvaluate_Deterministic(t *testing.T) {
	evaluator := NewEvaluator()

	input := DecisionInput{
		PositiveOutcomePct:    10.0,
		MedianOutcome:         0.05,
		RealisticMean:         0.08,
		RealisticMedian:       0.05,
		PessimisticMean:       0.04,
		PessimisticMedian:     0.03,
		OutcomeP10:            -0.02,
		OutcomeP25:            0.02,
		OutcomeP50:            0.05,
		OutcomeP75:            0.10,
		OutcomeP90:            0.15,
		StrategyImplementable: true,
		StrategyID:            "TIME_EXIT",
		EntryEventType:        "NEW_TOKEN",
		ScenarioID:            domain.ScenarioRealistic,
	}

	// Run multiple times
	var firstResult *DecisionResult
	for run := 0; run < 5; run++ {
		result, err := evaluator.Evaluate(input)
		if err != nil {
			t.Fatalf("Run %d: Evaluate failed: %v", run, err)
		}

		if firstResult == nil {
			firstResult = result
			continue
		}

		// Verify same decision
		if result.Decision != firstResult.Decision {
			t.Errorf("Run %d: Decision mismatch", run)
		}

		// Verify same GO criteria
		for i := range result.GOCriteria {
			if result.GOCriteria[i].Pass != firstResult.GOCriteria[i].Pass {
				t.Errorf("Run %d: GOCriteria[%d] mismatch", run, i)
			}
			if result.GOCriteria[i].Actual != firstResult.GOCriteria[i].Actual {
				t.Errorf("Run %d: GOCriteria[%d] Actual mismatch", run, i)
			}
		}

		// Verify same NO-GO checks
		for i := range result.NOGOChecks {
			if result.NOGOChecks[i].Pass != firstResult.NOGOChecks[i].Pass {
				t.Errorf("Run %d: NOGOChecks[%d] mismatch", run, i)
			}
		}
	}
}

func TestEvaluate_ValidationError(t *testing.T) {
	evaluator := NewEvaluator()

	// Missing StrategyID
	input := DecisionInput{
		PositiveOutcomePct: 10.0,
		ScenarioID:         domain.ScenarioRealistic,
	}

	_, err := evaluator.Evaluate(input)
	if err == nil {
		t.Error("Expected validation error for empty StrategyID")
	}

	// Non-realistic scenario
	input = DecisionInput{
		PositiveOutcomePct: 10.0,
		StrategyID:         "TEST",
		ScenarioID:         domain.ScenarioDegraded,
	}

	_, err = evaluator.Evaluate(input)
	if err == nil {
		t.Error("Expected validation error for non-realistic scenario")
	}
}

func TestRenderMarkdown_GO(t *testing.T) {
	result := &DecisionResult{
		Decision: DecisionGO,
		GOCriteria: []CriterionResult{
			{Name: "Positive outcome", Threshold: ">= 5%", Actual: "10.00%", Pass: true},
			{Name: "Median outcome", Threshold: "> 0", Actual: "0.0500", Pass: true},
			{Name: "Stability", Threshold: "ratio >= 0.5", Actual: "0.62", Pass: true},
			{Name: "Outliers", Threshold: "P50 > 0", Actual: "0.0500", Pass: true},
			{Name: "Implementable", Threshold: "true", Actual: "true", Pass: true},
		},
		NOGOChecks: []CriterionResult{
			{Name: "Low positive", Threshold: "< 5%", Actual: "10.00%", Pass: true},
			{Name: "Negative median", Threshold: "<= 0", Actual: "0.0500", Pass: true},
			{Name: "Edge disappears", Threshold: "degraded <= 0", Actual: "0.0500", Pass: true},
			{Name: "Not implementable", Threshold: "false", Actual: "true", Pass: true},
		},
	}

	md := RenderMarkdown(result)

	// Verify sections exist
	if !strings.Contains(md, "## Decision: GO") {
		t.Error("Markdown should contain GO decision")
	}
	if !strings.Contains(md, "## GO Criteria") {
		t.Error("Markdown should contain GO Criteria section")
	}
	if !strings.Contains(md, "## NO-GO Triggers") {
		t.Error("Markdown should contain NO-GO Triggers section")
	}
	if !strings.Contains(md, "5/5 passed") {
		t.Error("Markdown should show 5/5 GO criteria passed")
	}
	if !strings.Contains(md, "0/4 triggered") {
		t.Error("Markdown should show 0/4 NO-GO triggers")
	}
}

func TestRenderMarkdown_NOGO(t *testing.T) {
	result := &DecisionResult{
		Decision: DecisionNOGO,
		GOCriteria: []CriterionResult{
			{Name: "Positive outcome", Threshold: ">= 5%", Actual: "3.00%", Pass: false},
			{Name: "Median outcome", Threshold: "> 0", Actual: "0.0500", Pass: true},
			{Name: "Stability", Threshold: "ratio >= 0.5", Actual: "0.62", Pass: true},
			{Name: "Outliers", Threshold: "P50 > 0", Actual: "0.0500", Pass: true},
			{Name: "Implementable", Threshold: "true", Actual: "true", Pass: true},
		},
		NOGOChecks: []CriterionResult{
			{Name: "Low positive", Threshold: "< 5%", Actual: "3.00%", Pass: false}, // triggered
			{Name: "Negative median", Threshold: "<= 0", Actual: "0.0500", Pass: true},
			{Name: "Edge disappears", Threshold: "degraded <= 0", Actual: "0.0500", Pass: true},
			{Name: "Not implementable", Threshold: "false", Actual: "true", Pass: true},
		},
	}

	md := RenderMarkdown(result)

	if !strings.Contains(md, "## Decision: NO-GO") {
		t.Error("Markdown should contain NO-GO decision")
	}
	if !strings.Contains(md, "FAIL") {
		t.Error("Markdown should contain FAIL for failed criterion")
	}
	if !strings.Contains(md, "TRIGGERED") {
		t.Error("Markdown should contain TRIGGERED for triggered check")
	}
}
