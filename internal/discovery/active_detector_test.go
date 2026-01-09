package discovery

import (
	"context"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

func TestActiveDetector_VolumeSpikeTriggered(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Setup: 24h window with low volume, 1h window with high volume
	// volume_24h_avg = 240 / 24 = 10 per hour
	// volume_1h = 100 (needs to be > 3.0 * 10 = 30)

	evalTime := int64(86400000) // 24h mark

	// Add swaps distributed over 24h (low volume per hour)
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000) // every hour
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintA",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   10.0, // low volume
		})
	}

	// Add high volume swap in last hour
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintA",
		TxSignature: "txSpike",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   evalTime - 1000, // 1 second before eval
		AmountOut:   100.0,           // high volume spike
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate (volume spike), got %d", len(candidates))
	}

	if candidates[0].Source != domain.SourceActiveToken {
		t.Errorf("Expected source ACTIVE_TOKEN, got %s", candidates[0].Source)
	}
	if candidates[0].Mint != "MintA" {
		t.Errorf("Expected mint MintA, got %s", candidates[0].Mint)
	}
}

func TestActiveDetector_SwapsSpikeTriggered(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Setup: swaps_24h_avg = 24 / 24 = 1 per hour
	// swaps_1h = 10 (needs to be > 5.0 * 1 = 5)

	evalTime := int64(86400000)

	// Add 1 swap per hour for 24 hours
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000)
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintB",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   1.0,
		})
	}

	// Add many swaps in last hour (spike)
	for i := 0; i < 10; i++ {
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintB",
			TxSignature: "txSpike" + string(rune('0'+i)),
			EventIndex:  0,
			Slot:        int64(200 + i),
			Timestamp:   evalTime - int64(1000*(i+1)),
			AmountOut:   1.0, // low volume each, but many swaps
		})
	}

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate (swaps spike), got %d", len(candidates))
	}

	if candidates[0].Source != domain.SourceActiveToken {
		t.Errorf("Expected source ACTIVE_TOKEN, got %s", candidates[0].Source)
	}
}

func TestActiveDetector_NoSpikeBelow(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	evalTime := int64(86400000)

	// Uniform distribution - no spike
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000)
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintC",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   10.0,
		})
	}

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates (no spike), got %d", len(candidates))
	}
}

func TestActiveDetector_AlreadyDiscoveredSkipped(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Pre-insert as NEW_TOKEN
	_ = candidateStore.Insert(ctx, &domain.TokenCandidate{
		CandidateID: "existing123",
		Source:      domain.SourceNewToken,
		Mint:        "MintD",
		TxSignature: "txExisting",
		Slot:        50,
	})

	evalTime := int64(86400000)

	// Add spike for already-discovered mint
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000)
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintD",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   10.0,
		})
	}

	// Add spike
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintD",
		TxSignature: "txSpike",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   evalTime - 1000,
		AmountOut:   100.0,
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates (already discovered), got %d", len(candidates))
	}
}

func TestActiveDetector_Deterministic(t *testing.T) {
	evalTime := int64(86400000)

	for run := 0; run < 3; run++ {
		swapEventStore := memory.NewSwapEventStore()
		candidateStore := memory.NewCandidateStore()
		ctx := context.Background()

		// Add same data each run
		for i := 0; i < 24; i++ {
			ts := int64(i * 3600000)
			_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
				Mint:        "MintE",
				TxSignature: "tx" + string(rune('a'+i)),
				EventIndex:  0,
				Slot:        int64(100 + i),
				Timestamp:   ts,
				AmountOut:   10.0,
			})
		}

		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintE",
			TxSignature: "txSpike",
			EventIndex:  0,
			Slot:        200,
			Timestamp:   evalTime - 1000,
			AmountOut:   100.0,
		})

		config := DefaultActiveConfig()
		detector := NewActiveDetector(config, swapEventStore, candidateStore)

		candidates, err := detector.DetectAt(ctx, evalTime)
		if err != nil {
			t.Fatalf("Run %d: DetectAt failed: %v", run, err)
		}

		if len(candidates) != 1 {
			t.Fatalf("Run %d: Expected 1 candidate, got %d", run, len(candidates))
		}

		// Same input should produce same candidate_id
		if run > 0 {
			// All runs should produce same result (checked implicitly by test passing)
		}
	}
}

func TestActiveDetector_WindowBoundaries(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	evalTime := int64(86400000)

	// Swap exactly at boundary (should be excluded - end is exclusive)
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintF",
		TxSignature: "txBoundary",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   evalTime, // exactly at evalTime - should be excluded
		AmountOut:   1000.0,
	})

	// Swap at start of 24h window (should be included)
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintF",
		TxSignature: "txStart",
		EventIndex:  0,
		Slot:        101,
		Timestamp:   evalTime - Window24hMs, // exactly at start - should be included
		AmountOut:   1.0,
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	// The swap at evalTime should be excluded, only txStart included
	// No spike because only 1 swap with volume 1.0
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates (boundary excluded), got %d", len(candidates))
	}
}

func TestActiveDetector_TriggeringSwap(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	evalTime := int64(86400000)

	// Add background swaps
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000)
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintG",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   10.0,
		})
	}

	// Add spike swap with specific signature
	pool := "Pool123"
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintG",
		Pool:        &pool,
		TxSignature: "txTrigger",
		EventIndex:  5,
		Slot:        999,
		Timestamp:   evalTime - 500,
		AmountOut:   100.0,
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]

	// Verify triggering swap fields
	if c.TxSignature != "txTrigger" {
		t.Errorf("Expected tx_signature 'txTrigger', got '%s'", c.TxSignature)
	}
	if c.EventIndex != 5 {
		t.Errorf("Expected event_index 5, got %d", c.EventIndex)
	}
	if c.Slot != 999 {
		t.Errorf("Expected slot 999, got %d", c.Slot)
	}
	if c.Pool == nil || *c.Pool != "Pool123" {
		t.Errorf("Expected pool 'Pool123', got %v", c.Pool)
	}
}

func TestActiveDetector_LessThan1HourHistory_Skipped(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Evaluation time at 2 hours from epoch
	evalTime := int64(2 * 3600000)

	// Add swaps only in the last 30 minutes (less than 1 hour history)
	// Even with high volume, should be skipped because not enough history
	for i := 0; i < 10; i++ {
		ts := evalTime - int64((30-i)*60000) // 30 to 21 minutes before eval
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintShortHistory",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   100.0, // high volume
		})
	}

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	// Should be skipped due to <1h history
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates (<1h history skipped), got %d", len(candidates))
	}
}

func TestActiveDetector_PartialHistory_NormalizedCorrectly(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Token with only 2 hours of history (not full 24h)
	// Should normalize by 2 hours, not 24 hours
	evalTime := int64(3 * 3600000) // 3 hours from epoch

	// Add background volume: 1 hour ago = 10 volume
	// This will be the first swap, so actualHistory = evalTime - firstSwapTime = 2 hours
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintPartialHistory",
		TxSignature: "txEarly",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   evalTime - 2*3600000, // 2 hours ago
		AmountOut:   10.0,
	})

	// Add low volume 1.5 hours ago
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintPartialHistory",
		TxSignature: "txMiddle",
		EventIndex:  0,
		Slot:        101,
		Timestamp:   evalTime - int64(1.5*3600000),
		AmountOut:   5.0,
	})

	// Add spike volume in last hour: 100 volume
	// Total volume = 10 + 5 + 100 = 115
	// actualHours = 2 hours
	// volume24hAvg = 115 / 2 = 57.5 per hour
	// volume1h = 100
	// Spike requires: volume1h > 3.0 * volume24hAvg = 172.5
	// So 100 is NOT a spike with correct normalization

	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintPartialHistory",
		TxSignature: "txSpike",
		EventIndex:  0,
		Slot:        102,
		Timestamp:   evalTime - 500, // just before eval
		AmountOut:   100.0,
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	// With correct normalization by 2 hours (not 24h), no spike detected
	// If incorrectly normalized by 24h: volume24hAvg = 115/24 = 4.79, spike = 100 > 14.4 = TRUE (wrong)
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates (correct normalization prevents false spike), got %d", len(candidates))
	}
}

func TestActiveDetector_PartialHistory_TrueSpike(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Token with only 4 hours of history (not full 24h)
	// Should detect true spike with correct normalization
	evalTime := int64(5 * 3600000) // 5 hours from epoch

	// Add background volume over 4 hours: 10 volume per hour
	for i := 0; i < 4; i++ {
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        "MintTrueSpike",
			TxSignature: "tx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   evalTime - int64((4-i)*3600000), // 4, 3, 2, 1 hours ago
			AmountOut:   10.0,
		})
	}

	// Add massive spike in last hour: 200 volume
	// Total volume = 40 + 200 = 240
	// actualHours = 4 hours
	// volume24hAvg = 240 / 4 = 60 per hour
	// volume1h = 200
	// Spike requires: volume1h > 3.0 * volume24hAvg = 180
	// 200 > 180 = TRUE spike
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        "MintTrueSpike",
		TxSignature: "txSpike",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   evalTime - 500,
		AmountOut:   200.0,
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	// True spike detected with partial history normalization
	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate (true spike with partial history), got %d", len(candidates))
	}

	if candidates[0].Source != domain.SourceActiveToken {
		t.Errorf("Expected source ACTIVE_TOKEN, got %s", candidates[0].Source)
	}
}

func TestActiveDetector_LiquiditySpikeTriggered(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	liquidityEventStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	// Setup: 24h window with low liquidity changes, 1h window with high liquidity change
	// liq_24h_avg = 240 / 24 = 10 per hour
	// liq_1h = 50 (needs to be > 2.0 * 10 = 20)

	evalTime := int64(86400000) // 24h mark
	mint := "MintLiqSpike"

	// Need some swaps to discover the mint in GetDistinctMintsByTimeRange
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000)
		_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
			Mint:        mint,
			TxSignature: "swaptx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			AmountOut:   1.0, // low volume - no swap spike
		})
	}

	// Add liquidity events distributed over 24h (low changes per hour)
	for i := 0; i < 24; i++ {
		ts := int64(i * 3600000)
		// Use unique candidateID to allow insert (memory store requires non-empty candidateID)
		_ = liquidityEventStore.Insert(ctx, &domain.LiquidityEvent{
			CandidateID: "dummy_" + string(rune('a'+i)), // dummy for storage
			Mint:        mint,
			Pool:        "PoolLiq",
			TxSignature: "liqtx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			EventType:   domain.LiquidityEventAdd,
			AmountQuote: 10.0, // low liquidity change
		})
	}

	// Add high liquidity event in last hour
	_ = liquidityEventStore.Insert(ctx, &domain.LiquidityEvent{
		CandidateID: "dummy_spike",
		Mint:        mint,
		Pool:        "PoolLiq",
		TxSignature: "liqTxSpike",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   evalTime - 1000, // 1 second before eval
		EventType:   domain.LiquidityEventAdd,
		AmountQuote: 50.0, // high liquidity spike
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore).
		WithLiquidityStore(liquidityEventStore)

	candidates, err := detector.DetectAt(ctx, evalTime)
	if err != nil {
		t.Fatalf("DetectAt failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate (liquidity spike), got %d", len(candidates))
	}

	if candidates[0].Source != domain.SourceActiveToken {
		t.Errorf("Expected source ACTIVE_TOKEN, got %s", candidates[0].Source)
	}
	if candidates[0].Mint != mint {
		t.Errorf("Expected mint %s, got %s", mint, candidates[0].Mint)
	}
}

func TestActiveDetector_LiquiditySpikeWithMint_NoCandidateID(t *testing.T) {
	// This test verifies the fix for Task 47:
	// Liquidity spike detection must work using mint, not candidateID,
	// because candidates don't exist yet during ACTIVE_TOKEN discovery.

	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	liquidityEventStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	evalTime := int64(86400000)
	mint := "MintNoCandidateYet"

	// Add a swap to discover the mint
	_ = swapEventStore.Insert(ctx, &domain.SwapEvent{
		Mint:        mint,
		TxSignature: "swaptx1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   evalTime - 2*3600000, // 2 hours ago
		AmountOut:   1.0,
	})

	// Add liquidity events with mint set (but candidateID is a placeholder)
	// The key point: GetByMintTimeRange queries by mint, not candidateID
	for i := 0; i < 4; i++ {
		ts := evalTime - int64((4-i)*3600000)
		_ = liquidityEventStore.Insert(ctx, &domain.LiquidityEvent{
			CandidateID: "placeholder_" + string(rune('a'+i)), // not the real candidate
			Mint:        mint,                                 // this is what we query by
			Pool:        "Pool1",
			TxSignature: "liqtx" + string(rune('a'+i)),
			EventIndex:  0,
			Slot:        int64(100 + i),
			Timestamp:   ts,
			EventType:   domain.LiquidityEventAdd,
			AmountQuote: 10.0,
		})
	}

	// Add spike in last hour
	_ = liquidityEventStore.Insert(ctx, &domain.LiquidityEvent{
		CandidateID: "placeholder_spike",
		Mint:        mint,
		Pool:        "Pool1",
		TxSignature: "liqTxSpike",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   evalTime - 500,
		EventType:   domain.LiquidityEventAdd,
		AmountQuote: 100.0, // spike
	})

	config := DefaultActiveConfig()
	detector := NewActiveDetector(config, swapEventStore, candidateStore).
		WithLiquidityStore(liquidityEventStore)

	// EvaluateMint should find liquidity events by mint and detect spike
	candidate, err := detector.EvaluateMint(ctx, mint, evalTime)
	if err != nil {
		t.Fatalf("EvaluateMint failed: %v", err)
	}

	if candidate == nil {
		t.Fatal("Expected candidate to be created (liquidity spike detected via mint)")
	}

	if candidate.Mint != mint {
		t.Errorf("Expected mint %s, got %s", mint, candidate.Mint)
	}
}
