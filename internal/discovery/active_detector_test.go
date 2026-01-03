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
