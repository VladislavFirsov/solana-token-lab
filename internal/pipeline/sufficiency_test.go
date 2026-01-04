package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

func TestSufficiencyChecker_AllPass(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()

	// Create 300+ NEW_TOKEN candidates across 7+ days
	now := time.Now().UTC()
	for i := 0; i < 350; i++ {
		dayOffset := i % 10 // spread across 10 days
		candidate := &domain.TokenCandidate{
			CandidateID:  "cand_" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Source:       domain.SourceNewToken,
			Mint:         "mint_" + string(rune('A'+i%26)),
			TxSignature:  "tx_" + string(rune('A'+i%26)),
			EventIndex:   0,
			Slot:         int64(1000 + i),
			DiscoveredAt: now.AddDate(0, 0, -dayOffset).UnixMilli(),
			CreatedAt:    now.UnixMilli(),
		}
		if err := candidateStore.Insert(ctx, candidate); err != nil {
			t.Fatalf("Failed to insert candidate: %v", err)
		}

		// Add at least one swap per candidate
		swap := &domain.Swap{
			CandidateID: candidate.CandidateID,
			TxSignature: "swap_tx_" + candidate.CandidateID,
			EventIndex:  0,
			Slot:        candidate.Slot,
			Timestamp:   candidate.DiscoveredAt,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100,
			Price:       0.01,
		}
		if err := swapStore.Insert(ctx, swap); err != nil {
			t.Fatalf("Failed to insert swap: %v", err)
		}

		// Add at least one liquidity event per candidate
		liquidity := &domain.LiquidityEvent{
			CandidateID:    candidate.CandidateID,
			TxSignature:    "liq_tx_" + candidate.CandidateID,
			EventIndex:     0,
			Slot:           candidate.Slot,
			Timestamp:      candidate.DiscoveredAt,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    1000,
			AmountQuote:    10,
			LiquidityAfter: 1010,
		}
		if err := liquidityStore.Insert(ctx, liquidity); err != nil {
			t.Fatalf("Failed to insert liquidity event: %v", err)
		}
	}

	// Create trades spanning 14+ days using domain constants
	for i := 0; i < 100; i++ {
		dayOffset := i % 20 // spread across 20 days
		trade := &domain.TradeRecord{
			TradeID:         "trade_" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			CandidateID:     "cand_" + string(rune('A'+i%26)) + string(rune('0'+i/26%10)),
			StrategyID:      domain.StrategyTypeTimeExit,
			ScenarioID:      domain.ScenarioRealistic,
			EntrySignalTime: now.AddDate(0, 0, -dayOffset).UnixMilli(),
			EntryActualTime: now.AddDate(0, 0, -dayOffset).UnixMilli() + 1000,
			ExitSignalTime:  now.AddDate(0, 0, -dayOffset).UnixMilli() + 60000,
			ExitActualTime:  now.AddDate(0, 0, -dayOffset).UnixMilli() + 61000,
			ExitReason:      domain.ExitReasonTimeExit,
			Outcome:         0.05,
			OutcomeClass:    domain.OutcomeClassWin,
		}
		if err := tradeStore.Insert(ctx, trade); err != nil {
			t.Fatalf("Failed to insert trade: %v", err)
		}
	}

	checker := NewSufficiencyChecker(candidateStore, tradeStore, swapStore, liquidityStore, nil)
	result, err := checker.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Replayability will fail because runner is nil, so we expect AllPass=false
	// But other checks should work correctly
	if len(result.Checks) != 6 {
		t.Errorf("Expected 6 checks, got %d", len(result.Checks))
	}

	// Verify specific checks pass (except replayability)
	for _, check := range result.Checks {
		if check.Name == "Replayable tokens" {
			if check.Pass {
				t.Error("Expected 'Replayable tokens' check to fail when runner is nil")
			}
		} else if !check.Pass {
			t.Errorf("Expected check '%s' to pass, got actual=%s", check.Name, check.Actual)
		}
	}
}

func TestSufficiencyChecker_InsufficientCandidates(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()

	// Create only 100 NEW_TOKEN candidates (less than 300)
	now := time.Now().UTC()
	for i := 0; i < 100; i++ {
		candidate := &domain.TokenCandidate{
			CandidateID:  "cand_" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Source:       domain.SourceNewToken,
			Mint:         "mint_" + string(rune('A'+i%26)),
			TxSignature:  "tx_" + string(rune('A'+i%26)),
			EventIndex:   0,
			Slot:         int64(1000 + i),
			DiscoveredAt: now.AddDate(0, 0, -i%10).UnixMilli(),
			CreatedAt:    now.UnixMilli(),
		}
		if err := candidateStore.Insert(ctx, candidate); err != nil {
			t.Fatalf("Failed to insert candidate: %v", err)
		}
	}

	checker := NewSufficiencyChecker(candidateStore, tradeStore, swapStore, liquidityStore, nil)
	result, err := checker.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.AllPass {
		t.Error("Expected AllPass=false due to insufficient candidates")
	}

	// Find the unique candidates check
	var foundFailed bool
	for _, check := range result.Checks {
		if check.Name == "Unique NEW_TOKEN candidates" && !check.Pass {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Error("Expected 'Unique NEW_TOKEN candidates' check to fail")
	}
}

func TestSufficiencyChecker_InsufficientUptime(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()

	// Create 300 candidates but all on the same day (less than 7 days uptime)
	now := time.Now().UTC()
	for i := 0; i < 300; i++ {
		candidate := &domain.TokenCandidate{
			CandidateID:  "cand_" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Source:       domain.SourceNewToken,
			Mint:         "mint_" + string(rune('A'+i%26)),
			TxSignature:  "tx_" + string(rune('A'+i%26)),
			EventIndex:   0,
			Slot:         int64(1000 + i),
			DiscoveredAt: now.UnixMilli(), // All on same day
			CreatedAt:    now.UnixMilli(),
		}
		if err := candidateStore.Insert(ctx, candidate); err != nil {
			t.Fatalf("Failed to insert candidate: %v", err)
		}
	}

	checker := NewSufficiencyChecker(candidateStore, tradeStore, swapStore, liquidityStore, nil)
	result, err := checker.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if result.AllPass {
		t.Error("Expected AllPass=false due to insufficient uptime")
	}

	// Find the uptime check
	var foundFailed bool
	for _, check := range result.Checks {
		if check.Name == "Discovery uptime" && !check.Pass {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Error("Expected 'Discovery uptime' check to fail")
	}
}

func TestSufficiencyChecker_DuplicateCandidates(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()

	now := time.Now().UTC()

	// Insert one candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  "cand_A",
		Source:       domain.SourceNewToken,
		Mint:         "mint_A",
		TxSignature:  "tx_A",
		EventIndex:   0,
		Slot:         1000,
		DiscoveredAt: now.UnixMilli(),
		CreatedAt:    now.UnixMilli(),
	}
	if err := candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("Failed to insert candidate: %v", err)
	}

	// Note: Memory store prevents duplicates, so this test validates that
	// the checker correctly identifies zero duplicates when store enforces uniqueness
	checker := NewSufficiencyChecker(candidateStore, tradeStore, swapStore, liquidityStore, nil)
	result, err := checker.Check(ctx)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Find the duplicate check - should pass since we can't insert duplicates
	for _, check := range result.Checks {
		if check.Name == "Duplicate candidate_id count" {
			if !check.Pass {
				t.Error("Expected 'Duplicate candidate_id count' check to pass with unique candidates")
			}
			break
		}
	}
}

func TestPipeline_InsufficientDataDecision(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()

	// Create only 10 candidates (far below 300 threshold)
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		candidate := &domain.TokenCandidate{
			CandidateID:  "cand_" + string(rune('A'+i)),
			Source:       domain.SourceNewToken,
			Mint:         "mint_" + string(rune('A'+i)),
			TxSignature:  "tx_" + string(rune('A'+i)),
			EventIndex:   0,
			Slot:         int64(1000 + i),
			DiscoveredAt: now.AddDate(0, 0, -i).UnixMilli(),
			CreatedAt:    now.UnixMilli(),
		}
		if err := candidateStore.Insert(ctx, candidate); err != nil {
			t.Fatalf("Failed to insert candidate: %v", err)
		}
	}

	// Create temporary directory
	tempDir := t.TempDir()

	// Create pipeline with sufficiency checker
	p := NewPhase1Pipeline(
		candidateStore,
		tradeStore,
		aggStore,
		nil, // no implementable strategies
		tempDir,
	).WithSufficiencyChecker(
		candidateStore,
		tradeStore,
		swapStore,
		liquidityStore,
		nil, // no replay runner
	).WithClock(func() time.Time { return now })

	// Run pipeline
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Pipeline run failed: %v", err)
	}

	// Read decision report
	decisionData, err := readFile(tempDir, "DECISION_GATE_REPORT.md")
	if err != nil {
		t.Fatalf("Failed to read decision report: %v", err)
	}

	// Verify INSUFFICIENT_DATA decision
	if !contains(decisionData, "INSUFFICIENT_DATA") {
		t.Error("Expected decision report to contain INSUFFICIENT_DATA")
	}
	if !contains(decisionData, "Data sufficiency checks failed") {
		t.Error("Expected decision report to explain insufficient data")
	}
}

// readFile reads file contents from directory
func readFile(dir, filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
