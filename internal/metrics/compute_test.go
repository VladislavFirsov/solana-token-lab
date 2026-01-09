package metrics

import (
	"math"
	"testing"

	"solana-token-lab/internal/domain"
)

func TestComputeTokenWinRate_AllPositiveOutcomes(t *testing.T) {
	// Token with all positive outcomes → counted as winning
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "token-A", Outcome: 0.05},
		{TradeID: "t2", CandidateID: "token-A", Outcome: 0.10},
		{TradeID: "t3", CandidateID: "token-A", Outcome: 0.03},
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 1 {
		t.Errorf("expected totalTokens 1, got %d", totalTokens)
	}
	// Mean = (0.05 + 0.10 + 0.03) / 3 = 0.06 > 0 → winning
	if winRate != 1.0 {
		t.Errorf("expected winRate 1.0, got %f", winRate)
	}
}

func TestComputeTokenWinRate_AllNegativeOutcomes(t *testing.T) {
	// Token with all negative outcomes → not counted as winning
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "token-A", Outcome: -0.05},
		{TradeID: "t2", CandidateID: "token-A", Outcome: -0.10},
		{TradeID: "t3", CandidateID: "token-A", Outcome: -0.03},
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 1 {
		t.Errorf("expected totalTokens 1, got %d", totalTokens)
	}
	// Mean = (-0.05 + -0.10 + -0.03) / 3 = -0.06 ≤ 0 → not winning
	if winRate != 0.0 {
		t.Errorf("expected winRate 0.0, got %f", winRate)
	}
}

func TestComputeTokenWinRate_MixedPositiveMean(t *testing.T) {
	// Token with mixed outcomes, positive mean → counted as winning
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "token-A", Outcome: -0.02},
		{TradeID: "t2", CandidateID: "token-A", Outcome: 0.05},
		{TradeID: "t3", CandidateID: "token-A", Outcome: 0.03},
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 1 {
		t.Errorf("expected totalTokens 1, got %d", totalTokens)
	}
	// Mean = (-0.02 + 0.05 + 0.03) / 3 = 0.02 > 0 → winning
	if winRate != 1.0 {
		t.Errorf("expected winRate 1.0, got %f", winRate)
	}
}

func TestComputeTokenWinRate_MixedNegativeMean(t *testing.T) {
	// Token with mixed outcomes, negative mean → NOT counted as winning
	// This is the key case that distinguishes "positive mean" from "any positive"
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "token-A", Outcome: -0.05},
		{TradeID: "t2", CandidateID: "token-A", Outcome: -0.03},
		{TradeID: "t3", CandidateID: "token-A", Outcome: 0.01}, // Has a positive trade
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 1 {
		t.Errorf("expected totalTokens 1, got %d", totalTokens)
	}
	// Mean = (-0.05 + -0.03 + 0.01) / 3 = -0.0233... ≤ 0 → NOT winning
	// Note: With old "any positive" logic, this would have been counted as winning
	if winRate != 0.0 {
		t.Errorf("expected winRate 0.0 (negative mean despite one positive trade), got %f", winRate)
	}
}

func TestComputeTokenWinRate_ZeroMean(t *testing.T) {
	// Token with zero mean → NOT counted as winning (must be > 0, not >= 0)
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "token-A", Outcome: -0.05},
		{TradeID: "t2", CandidateID: "token-A", Outcome: 0.05},
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 1 {
		t.Errorf("expected totalTokens 1, got %d", totalTokens)
	}
	// Mean = (-0.05 + 0.05) / 2 = 0.0 → NOT winning (needs to be > 0)
	if winRate != 0.0 {
		t.Errorf("expected winRate 0.0 (zero mean is not winning), got %f", winRate)
	}
}

func TestComputeTokenWinRate_MultipleTokens(t *testing.T) {
	// Multiple tokens with different outcomes
	trades := []*domain.TradeRecord{
		// Token A: mean = (-0.05 + -0.03 + 0.01) / 3 = -0.0233 → NOT winning
		{TradeID: "t1", CandidateID: "token-A", Outcome: -0.05},
		{TradeID: "t2", CandidateID: "token-A", Outcome: -0.03},
		{TradeID: "t3", CandidateID: "token-A", Outcome: 0.01},

		// Token B: mean = (-0.02 + 0.05 + 0.03) / 3 = 0.02 → winning
		{TradeID: "t4", CandidateID: "token-B", Outcome: -0.02},
		{TradeID: "t5", CandidateID: "token-B", Outcome: 0.05},
		{TradeID: "t6", CandidateID: "token-B", Outcome: 0.03},

		// Token C: mean = (-0.10) / 1 = -0.10 → NOT winning
		{TradeID: "t7", CandidateID: "token-C", Outcome: -0.10},

		// Token D: mean = (0.20) / 1 = 0.20 → winning
		{TradeID: "t8", CandidateID: "token-D", Outcome: 0.20},
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 4 {
		t.Errorf("expected totalTokens 4, got %d", totalTokens)
	}
	// 2 winning tokens (B, D) out of 4 → 0.5 win rate
	expectedWinRate := 0.5
	if math.Abs(winRate-expectedWinRate) > 0.0001 {
		t.Errorf("expected winRate %.4f, got %.4f", expectedWinRate, winRate)
	}
}

func TestComputeTokenWinRate_EmptyTrades(t *testing.T) {
	totalTokens, winRate := computeTokenWinRate(nil)

	if totalTokens != 0 {
		t.Errorf("expected totalTokens 0, got %d", totalTokens)
	}
	if winRate != 0 {
		t.Errorf("expected winRate 0, got %f", winRate)
	}

	totalTokens, winRate = computeTokenWinRate([]*domain.TradeRecord{})

	if totalTokens != 0 {
		t.Errorf("expected totalTokens 0, got %d", totalTokens)
	}
	if winRate != 0 {
		t.Errorf("expected winRate 0, got %f", winRate)
	}
}

func TestComputeTokenWinRate_SingleTradePerToken(t *testing.T) {
	// Each token has exactly one trade
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "token-A", Outcome: 0.10},  // positive → winning
		{TradeID: "t2", CandidateID: "token-B", Outcome: -0.05}, // negative → not winning
		{TradeID: "t3", CandidateID: "token-C", Outcome: 0.00},  // zero → not winning
	}

	totalTokens, winRate := computeTokenWinRate(trades)

	if totalTokens != 3 {
		t.Errorf("expected totalTokens 3, got %d", totalTokens)
	}
	// 1 winning token (A) out of 3 → ~0.333 win rate
	expectedWinRate := 1.0 / 3.0
	if math.Abs(winRate-expectedWinRate) > 0.0001 {
		t.Errorf("expected winRate %.4f, got %.4f", expectedWinRate, winRate)
	}
}
