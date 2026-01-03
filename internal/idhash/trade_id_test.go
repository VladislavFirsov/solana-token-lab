package idhash

import (
	"testing"
)

func TestComputeTradeID(t *testing.T) {
	tests := []struct {
		name            string
		candidateID     string
		strategyID      string
		scenarioID      string
		entrySignalTime int64
		wantLen         int // hash length should be 64
	}{
		{
			name:            "basic trade",
			candidateID:     "abc123def456",
			strategyID:      "TIME_EXIT_NEW_TOKEN_300",
			scenarioID:      "realistic",
			entrySignalTime: 1704067234567,
			wantLen:         64,
		},
		{
			name:            "trailing stop trade",
			candidateID:     "xyz789ghi012",
			strategyID:      "TRAILING_STOP_ACTIVE_TOKEN_0.10",
			scenarioID:      "pessimistic",
			entrySignalTime: 1704067300000,
			wantLen:         64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeTradeID(tt.candidateID, tt.strategyID, tt.scenarioID, tt.entrySignalTime)

			if len(got) != tt.wantLen {
				t.Errorf("ComputeTradeID() length = %d, want %d", len(got), tt.wantLen)
			}

			// Verify determinism: same inputs should produce same output
			got2 := ComputeTradeID(tt.candidateID, tt.strategyID, tt.scenarioID, tt.entrySignalTime)
			if got != got2 {
				t.Errorf("ComputeTradeID() not deterministic: %s != %s", got, got2)
			}
		})
	}
}

func TestComputeTradeID_Determinism(t *testing.T) {
	candidateID := "candidate123"
	strategyID := "strategy456"
	scenarioID := "realistic"
	entryTime := int64(1704067234567)

	// Compute multiple times
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = ComputeTradeID(candidateID, strategyID, scenarioID, entryTime)
	}

	// All should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("Determinism failed: results[%d]=%s != results[0]=%s", i, results[i], results[0])
		}
	}
}

func TestComputeTradeID_DifferentInputs(t *testing.T) {
	base := ComputeTradeID("candidate", "strategy", "scenario", 1000)

	// Different candidate should produce different hash
	diffCandidate := ComputeTradeID("other_candidate", "strategy", "scenario", 1000)
	if base == diffCandidate {
		t.Error("Different candidate should produce different hash")
	}

	// Different strategy should produce different hash
	diffStrategy := ComputeTradeID("candidate", "other_strategy", "scenario", 1000)
	if base == diffStrategy {
		t.Error("Different strategy should produce different hash")
	}

	// Different scenario should produce different hash
	diffScenario := ComputeTradeID("candidate", "strategy", "other_scenario", 1000)
	if base == diffScenario {
		t.Error("Different scenario should produce different hash")
	}

	// Different entry time should produce different hash
	diffTime := ComputeTradeID("candidate", "strategy", "scenario", 2000)
	if base == diffTime {
		t.Error("Different entry time should produce different hash")
	}
}
