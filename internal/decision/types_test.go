package decision

import (
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
)

func TestDecisionInput_Validate(t *testing.T) {
	validInput := &DecisionInput{
		StrategyID:         "test-strategy",
		ScenarioID:         domain.ScenarioRealistic,
		PositiveOutcomePct: 50.0,
	}

	// Valid input
	if err := validInput.Validate(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Nil input
	var nilInput *DecisionInput
	if err := nilInput.Validate(); err == nil {
		t.Error("expected error for nil input")
	}

	// Empty strategy ID
	input := *validInput
	input.StrategyID = ""
	if err := input.Validate(); !errors.Is(err, ErrEmptyStrategyID) {
		t.Errorf("expected ErrEmptyStrategyID, got %v", err)
	}

	// Empty scenario ID
	input = *validInput
	input.ScenarioID = ""
	if err := input.Validate(); !errors.Is(err, ErrEmptyScenarioID) {
		t.Errorf("expected ErrEmptyScenarioID, got %v", err)
	}

	// Non-realistic scenario
	input = *validInput
	input.ScenarioID = domain.ScenarioDegraded
	if err := input.Validate(); !errors.Is(err, ErrNotRealisticScenario) {
		t.Errorf("expected ErrNotRealisticScenario, got %v", err)
	}

	// Negative outcome percentage
	input = *validInput
	input.PositiveOutcomePct = -1
	if err := input.Validate(); !errors.Is(err, ErrNegativeOutcomePct) {
		t.Errorf("expected ErrNegativeOutcomePct, got %v", err)
	}

	// Outcome percentage over 100
	input = *validInput
	input.PositiveOutcomePct = 101
	if err := input.Validate(); !errors.Is(err, ErrInvalidOutcomePct) {
		t.Errorf("expected ErrInvalidOutcomePct, got %v", err)
	}

	// Boundary cases - valid
	input = *validInput
	input.PositiveOutcomePct = 0
	if err := input.Validate(); err != nil {
		t.Errorf("0%% should be valid, got %v", err)
	}

	input.PositiveOutcomePct = 100
	if err := input.Validate(); err != nil {
		t.Errorf("100%% should be valid, got %v", err)
	}
}
