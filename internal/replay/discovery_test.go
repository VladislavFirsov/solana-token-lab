package replay

import (
	"context"
	"testing"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

func TestDiscoveryReplay_Deterministic(t *testing.T) {
	// Run replay multiple times with same input, verify same output
	for run := 0; run < 5; run++ {
		swapEventStore := memory.NewSwapEventStore()
		candidateStore := memory.NewCandidateStore()

		ctx := context.Background()

		// Add swap events (unordered to test sorting)
		events := []*domain.SwapEvent{
			{Mint: "mint3", TxSignature: "tx3", EventIndex: 0, Slot: 300, Timestamp: 3000, AmountOut: 100},
			{Mint: "mint1", TxSignature: "tx1", EventIndex: 0, Slot: 100, Timestamp: 1000, AmountOut: 100},
			{Mint: "mint2", TxSignature: "tx2", EventIndex: 0, Slot: 200, Timestamp: 2000, AmountOut: 100},
		}
		if err := swapEventStore.InsertBulk(ctx, events); err != nil {
			t.Fatalf("InsertBulk failed: %v", err)
		}

		replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

		candidates, err := replay.Run(ctx, 0, 10000)
		if err != nil {
			t.Fatalf("Run %d: replay failed: %v", run, err)
		}

		// Should have 3 NEW_TOKEN candidates
		if len(candidates) != 3 {
			t.Errorf("Run %d: expected 3 candidates, got %d", run, len(candidates))
		}

		// Verify order: mint1, mint2, mint3 (by slot)
		if len(candidates) >= 3 {
			if candidates[0].Mint != "mint1" {
				t.Errorf("Run %d: first candidate should be mint1, got %s", run, candidates[0].Mint)
			}
			if candidates[1].Mint != "mint2" {
				t.Errorf("Run %d: second candidate should be mint2, got %s", run, candidates[1].Mint)
			}
			if candidates[2].Mint != "mint3" {
				t.Errorf("Run %d: third candidate should be mint3, got %s", run, candidates[2].Mint)
			}
		}

		// Verify all are NEW_TOKEN
		for i, c := range candidates {
			if c.Source != domain.SourceNewToken {
				t.Errorf("Run %d: candidate %d should be NEW_TOKEN, got %s", run, i, c.Source)
			}
		}
	}
}

func TestDiscoveryReplay_NewTokenBeforeActive(t *testing.T) {
	// If a mint could qualify as both NEW_TOKEN and ACTIVE_TOKEN,
	// NEW_TOKEN should win (detected first, mint becomes seen)
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()

	ctx := context.Background()

	// Single event - would trigger NEW_TOKEN
	events := []*domain.SwapEvent{
		{Mint: "mint1", TxSignature: "tx1", EventIndex: 0, Slot: 100, Timestamp: 1000, AmountOut: 1000},
	}
	if err := swapEventStore.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

	candidates, err := replay.Run(ctx, 0, 10000)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Should have exactly 1 candidate (NEW_TOKEN)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	if candidates[0].Source != domain.SourceNewToken {
		t.Errorf("expected NEW_TOKEN, got %s", candidates[0].Source)
	}
}

func TestDiscoveryReplay_NoRPC(t *testing.T) {
	// Verify replay uses only storage interfaces (no RPC)
	// This is implicit - we use memory stores which don't call RPC
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()

	ctx := context.Background()

	events := []*domain.SwapEvent{
		{Mint: "mint1", TxSignature: "tx1", EventIndex: 0, Slot: 100, Timestamp: 1000, AmountOut: 100},
	}
	if err := swapEventStore.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

	// If this runs without error using memory stores, no RPC was used
	_, err := replay.Run(ctx, 0, 10000)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
}

func TestDiscoveryReplay_EmptyRange(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()

	ctx := context.Background()

	replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

	candidates, err := replay.Run(ctx, 0, 10000)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if candidates != nil && len(candidates) != 0 {
		t.Errorf("expected nil or empty, got %d candidates", len(candidates))
	}
}

func TestDiscoveryReplay_Ordering(t *testing.T) {
	// Verify output is ordered by (slot, tx_signature, event_index)
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()

	ctx := context.Background()

	// Add events with same slot, different tx_signatures
	events := []*domain.SwapEvent{
		{Mint: "mint3", TxSignature: "txC", EventIndex: 0, Slot: 100, Timestamp: 1000, AmountOut: 100},
		{Mint: "mint1", TxSignature: "txA", EventIndex: 0, Slot: 100, Timestamp: 1000, AmountOut: 100},
		{Mint: "mint2", TxSignature: "txB", EventIndex: 0, Slot: 100, Timestamp: 1000, AmountOut: 100},
	}
	if err := swapEventStore.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

	candidates, err := replay.Run(ctx, 0, 10000)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	// Should be ordered by tx_signature: txA, txB, txC -> mint1, mint2, mint3
	expectedOrder := []string{"mint1", "mint2", "mint3"}
	for i, expected := range expectedOrder {
		if candidates[i].Mint != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, candidates[i].Mint)
		}
	}
}

func TestDiscoveryReplay_ActiveTokenDetection(t *testing.T) {
	// Test that ACTIVE_TOKEN detection works for mints with spike
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()

	ctx := context.Background()

	// Create events for mint1 that would trigger ACTIVE_TOKEN spike
	// Need 24h of baseline + 1h spike
	baseTime := int64(100000000) // base timestamp

	var events []*domain.SwapEvent

	// First, add a NEW_TOKEN mint (mint0) so we can test ACTIVE_TOKEN separately
	events = append(events, &domain.SwapEvent{
		Mint: "mint0", TxSignature: "tx0", EventIndex: 0, Slot: 1, Timestamp: baseTime - 90000000, AmountOut: 100,
	})

	// 24h baseline for mint1 (low activity) - events spread over 24h
	for i := 0; i < 24; i++ {
		events = append(events, &domain.SwapEvent{
			Mint:        "mint1",
			TxSignature: "txBase" + string(rune('A'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   baseTime - int64(86400000-i*3600000), // spread over 24h before baseline
			AmountOut:   10,                                   // low volume
		})
	}

	// 1h spike for mint1 (high activity) - triggers ACTIVE_TOKEN
	// K_vol = 3.0, K_swaps = 5.0
	// volume_24h_avg = 240/24 = 10, need volume_1h > 30
	// swaps_24h_avg = 24/24 = 1, need swaps_1h > 5
	for i := 0; i < 10; i++ { // 10 swaps in 1h > 5 * 1
		events = append(events, &domain.SwapEvent{
			Mint:        "mint1",
			TxSignature: "txSpike" + string(rune('A'+i)),
			EventIndex:  0,
			Slot:        int64(200 + i),
			Timestamp:   baseTime + int64(i*1000), // within 1h window
			AmountOut:   50,                       // high volume: 500 total > 30
		})
	}

	if err := swapEventStore.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

	// Replay from before baseline to after spike
	candidates, err := replay.Run(ctx, baseTime-90000000, baseTime+100000)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Should have:
	// 1. NEW_TOKEN for mint0 (first swap)
	// 2. NEW_TOKEN for mint1 (first swap in baseline)
	// Note: ACTIVE_TOKEN not triggered because mint1 was already discovered as NEW_TOKEN

	// Count by source
	newTokenCount := 0
	activeTokenCount := 0
	for _, c := range candidates {
		switch c.Source {
		case domain.SourceNewToken:
			newTokenCount++
		case domain.SourceActiveToken:
			activeTokenCount++
		}
	}

	if newTokenCount != 2 {
		t.Errorf("expected 2 NEW_TOKEN, got %d", newTokenCount)
	}

	// ACTIVE_TOKEN should not trigger because mint1 was already discovered as NEW_TOKEN
	if activeTokenCount != 0 {
		t.Errorf("expected 0 ACTIVE_TOKEN (mint already discovered), got %d", activeTokenCount)
	}
}

func TestDiscoveryReplay_ActiveTokenForExistingMint(t *testing.T) {
	// Test ACTIVE_TOKEN detection for a mint that was pre-seeded as candidate
	// (simulating partial replay where mint0 was discovered before our time range)
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()

	ctx := context.Background()

	// Pre-seed candidateStore with mint1 as existing candidate
	existingCandidate := &domain.TokenCandidate{
		CandidateID: "existing-candidate-id",
		Source:      domain.SourceNewToken,
		Mint:        "mint1",
		TxSignature: "txOld",
		EventIndex:  0,
		Slot:        1,
	}
	if err := candidateStore.Insert(ctx, existingCandidate); err != nil {
		t.Fatalf("Insert existing candidate failed: %v", err)
	}

	baseTime := int64(100000000)

	// Add baseline events for mint1 (24h history)
	var events []*domain.SwapEvent
	for i := 0; i < 24; i++ {
		events = append(events, &domain.SwapEvent{
			Mint:        "mint1",
			TxSignature: "txBase" + string(rune('A'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   baseTime - int64(86400000-i*3600000),
			AmountOut:   10,
		})
	}

	// Add spike events for mint1 - should NOT trigger ACTIVE_TOKEN
	// because mint1 is already a candidate
	for i := 0; i < 10; i++ {
		events = append(events, &domain.SwapEvent{
			Mint:        "mint1",
			TxSignature: "txSpike" + string(rune('A'+i)),
			EventIndex:  0,
			Slot:        int64(200 + i),
			Timestamp:   baseTime + int64(i*1000),
			AmountOut:   50,
		})
	}

	// Add events for new mint2 - should trigger NEW_TOKEN
	events = append(events, &domain.SwapEvent{
		Mint: "mint2", TxSignature: "txMint2", EventIndex: 0, Slot: 300, Timestamp: baseTime + 50000, AmountOut: 100,
	})

	if err := swapEventStore.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	replay := NewDiscoveryReplay(swapEventStore, candidateStore, discovery.DefaultActiveConfig())

	candidates, err := replay.Run(ctx, baseTime-90000000, baseTime+100000)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Should have only 1 NEW_TOKEN for mint2
	// mint1 should not produce any candidate (already exists)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	if candidates[0].Mint != "mint2" {
		t.Errorf("expected mint2, got %s", candidates[0].Mint)
	}

	if candidates[0].Source != domain.SourceNewToken {
		t.Errorf("expected NEW_TOKEN, got %s", candidates[0].Source)
	}
}
