// Package verification implements trade replay verification per REPLAY_PROTOCOL.md Section 6.
// It verifies that stored trade records match replayed simulations.
package verification

import (
	"context"
	"math"

	"solana-token-lab/internal/domain"
)

// FloatTolerance is the tolerance for float64 comparisons.
// Per REPLAY_PROTOCOL.md Section 6.1: outcome within 0.0000001 tolerance.
const FloatTolerance = 1e-7

// FieldDivergence represents a mismatch between stored and replayed values.
type FieldDivergence struct {
	Field    string      // field name
	Expected interface{} // stored value
	Actual   interface{} // replayed value
}

// VerificationResult contains the result of verifying a single trade.
type VerificationResult struct {
	TradeID         string            // verified trade ID
	Match           bool              // true if all fields match
	Divergences     []FieldDivergence // list of divergent fields
	StoredOutcome   float64           // outcome from stored trade
	ReplayedOutcome float64           // outcome from replayed simulation
}

// VerificationReport contains results for batch verification.
type VerificationReport struct {
	TotalTrades     int                  // total trades verified
	MatchedTrades   int                  // trades that matched exactly
	DivergentTrades int                  // trades with divergences
	Results         []VerificationResult // individual results
}

// Verifier interface for trade replay verification.
type Verifier interface {
	// VerifyTrade verifies a single trade by ID.
	// It loads the stored trade, re-executes simulation with same parameters,
	// and compares all fields.
	VerifyTrade(ctx context.Context, tradeID string) (*VerificationResult, error)

	// VerifyAll verifies all stored trades.
	// Returns a report with individual results.
	VerifyAll(ctx context.Context) (*VerificationReport, error)
}

// CompareTradeRecords compares two trade records and returns divergences.
// Uses FloatTolerance for float64 comparisons.
func CompareTradeRecords(stored, replayed *domain.TradeRecord) []FieldDivergence {
	var divergences []FieldDivergence

	// TradeID must match exactly
	if stored.TradeID != replayed.TradeID {
		divergences = append(divergences, FieldDivergence{
			Field:    "TradeID",
			Expected: stored.TradeID,
			Actual:   replayed.TradeID,
		})
	}

	// CandidateID must match
	if stored.CandidateID != replayed.CandidateID {
		divergences = append(divergences, FieldDivergence{
			Field:    "CandidateID",
			Expected: stored.CandidateID,
			Actual:   replayed.CandidateID,
		})
	}

	// StrategyID must match
	if stored.StrategyID != replayed.StrategyID {
		divergences = append(divergences, FieldDivergence{
			Field:    "StrategyID",
			Expected: stored.StrategyID,
			Actual:   replayed.StrategyID,
		})
	}

	// ScenarioID must match
	if stored.ScenarioID != replayed.ScenarioID {
		divergences = append(divergences, FieldDivergence{
			Field:    "ScenarioID",
			Expected: stored.ScenarioID,
			Actual:   replayed.ScenarioID,
		})
	}

	// Entry values
	if stored.EntrySignalTime != replayed.EntrySignalTime {
		divergences = append(divergences, FieldDivergence{
			Field:    "EntrySignalTime",
			Expected: stored.EntrySignalTime,
			Actual:   replayed.EntrySignalTime,
		})
	}

	if !floatEquals(stored.EntrySignalPrice, replayed.EntrySignalPrice) {
		divergences = append(divergences, FieldDivergence{
			Field:    "EntrySignalPrice",
			Expected: stored.EntrySignalPrice,
			Actual:   replayed.EntrySignalPrice,
		})
	}

	if stored.EntryActualTime != replayed.EntryActualTime {
		divergences = append(divergences, FieldDivergence{
			Field:    "EntryActualTime",
			Expected: stored.EntryActualTime,
			Actual:   replayed.EntryActualTime,
		})
	}

	if !floatEquals(stored.EntryActualPrice, replayed.EntryActualPrice) {
		divergences = append(divergences, FieldDivergence{
			Field:    "EntryActualPrice",
			Expected: stored.EntryActualPrice,
			Actual:   replayed.EntryActualPrice,
		})
	}

	if !floatPtrEquals(stored.EntryLiquidity, replayed.EntryLiquidity) {
		divergences = append(divergences, FieldDivergence{
			Field:    "EntryLiquidity",
			Expected: stored.EntryLiquidity,
			Actual:   replayed.EntryLiquidity,
		})
	}

	if !floatEquals(stored.PositionSize, replayed.PositionSize) {
		divergences = append(divergences, FieldDivergence{
			Field:    "PositionSize",
			Expected: stored.PositionSize,
			Actual:   replayed.PositionSize,
		})
	}

	if !floatEquals(stored.PositionValue, replayed.PositionValue) {
		divergences = append(divergences, FieldDivergence{
			Field:    "PositionValue",
			Expected: stored.PositionValue,
			Actual:   replayed.PositionValue,
		})
	}

	// Exit values
	if stored.ExitSignalTime != replayed.ExitSignalTime {
		divergences = append(divergences, FieldDivergence{
			Field:    "ExitSignalTime",
			Expected: stored.ExitSignalTime,
			Actual:   replayed.ExitSignalTime,
		})
	}

	if !floatEquals(stored.ExitSignalPrice, replayed.ExitSignalPrice) {
		divergences = append(divergences, FieldDivergence{
			Field:    "ExitSignalPrice",
			Expected: stored.ExitSignalPrice,
			Actual:   replayed.ExitSignalPrice,
		})
	}

	if stored.ExitActualTime != replayed.ExitActualTime {
		divergences = append(divergences, FieldDivergence{
			Field:    "ExitActualTime",
			Expected: stored.ExitActualTime,
			Actual:   replayed.ExitActualTime,
		})
	}

	if !floatEquals(stored.ExitActualPrice, replayed.ExitActualPrice) {
		divergences = append(divergences, FieldDivergence{
			Field:    "ExitActualPrice",
			Expected: stored.ExitActualPrice,
			Actual:   replayed.ExitActualPrice,
		})
	}

	// ExitReason must match exactly
	if stored.ExitReason != replayed.ExitReason {
		divergences = append(divergences, FieldDivergence{
			Field:    "ExitReason",
			Expected: stored.ExitReason,
			Actual:   replayed.ExitReason,
		})
	}

	// Cost values
	if !floatEquals(stored.EntryCostSOL, replayed.EntryCostSOL) {
		divergences = append(divergences, FieldDivergence{
			Field:    "EntryCostSOL",
			Expected: stored.EntryCostSOL,
			Actual:   replayed.EntryCostSOL,
		})
	}

	if !floatEquals(stored.ExitCostSOL, replayed.ExitCostSOL) {
		divergences = append(divergences, FieldDivergence{
			Field:    "ExitCostSOL",
			Expected: stored.ExitCostSOL,
			Actual:   replayed.ExitCostSOL,
		})
	}

	if !floatEquals(stored.MEVCostSOL, replayed.MEVCostSOL) {
		divergences = append(divergences, FieldDivergence{
			Field:    "MEVCostSOL",
			Expected: stored.MEVCostSOL,
			Actual:   replayed.MEVCostSOL,
		})
	}

	if !floatEquals(stored.TotalCostSOL, replayed.TotalCostSOL) {
		divergences = append(divergences, FieldDivergence{
			Field:    "TotalCostSOL",
			Expected: stored.TotalCostSOL,
			Actual:   replayed.TotalCostSOL,
		})
	}

	if !floatEquals(stored.TotalCostPct, replayed.TotalCostPct) {
		divergences = append(divergences, FieldDivergence{
			Field:    "TotalCostPct",
			Expected: stored.TotalCostPct,
			Actual:   replayed.TotalCostPct,
		})
	}

	// Outcome values (critical for verification)
	if !floatEquals(stored.GrossReturn, replayed.GrossReturn) {
		divergences = append(divergences, FieldDivergence{
			Field:    "GrossReturn",
			Expected: stored.GrossReturn,
			Actual:   replayed.GrossReturn,
		})
	}

	if !floatEquals(stored.Outcome, replayed.Outcome) {
		divergences = append(divergences, FieldDivergence{
			Field:    "Outcome",
			Expected: stored.Outcome,
			Actual:   replayed.Outcome,
		})
	}

	if stored.OutcomeClass != replayed.OutcomeClass {
		divergences = append(divergences, FieldDivergence{
			Field:    "OutcomeClass",
			Expected: stored.OutcomeClass,
			Actual:   replayed.OutcomeClass,
		})
	}

	// Metadata
	if stored.HoldDurationMs != replayed.HoldDurationMs {
		divergences = append(divergences, FieldDivergence{
			Field:    "HoldDurationMs",
			Expected: stored.HoldDurationMs,
			Actual:   replayed.HoldDurationMs,
		})
	}

	if !floatPtrEquals(stored.PeakPrice, replayed.PeakPrice) {
		divergences = append(divergences, FieldDivergence{
			Field:    "PeakPrice",
			Expected: stored.PeakPrice,
			Actual:   replayed.PeakPrice,
		})
	}

	if !floatPtrEquals(stored.MinLiquidity, replayed.MinLiquidity) {
		divergences = append(divergences, FieldDivergence{
			Field:    "MinLiquidity",
			Expected: stored.MinLiquidity,
			Actual:   replayed.MinLiquidity,
		})
	}

	return divergences
}

// floatEquals compares two float64 values within FloatTolerance.
// Per REPLAY_PROTOCOL.md Section 6.1: tolerance = 1e-7 exactly.
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) <= FloatTolerance
}

// floatPtrEquals compares two *float64 values within FloatTolerance.
// Returns true if both are nil, or both are non-nil and equal.
func floatPtrEquals(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return floatEquals(*a, *b)
}
