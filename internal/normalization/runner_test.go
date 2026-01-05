package normalization

import (
	"context"
	"math"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

func TestGeneratePriceTimeseries_Basic(t *testing.T) {
	swaps := []*domain.Swap{
		{CandidateID: "c1", Timestamp: 1000, Slot: 100, Price: 1.0, AmountOut: 10.0},
		{CandidateID: "c1", Timestamp: 2000, Slot: 200, Price: 2.0, AmountOut: 20.0},
	}

	result := GeneratePriceTimeseries(swaps)

	if len(result) != 2 {
		t.Fatalf("Expected 2 points, got %d", len(result))
	}

	if result[0].Price != 1.0 || result[0].Volume != 10.0 || result[0].SwapCount != 1 {
		t.Errorf("Point 0: expected (1.0, 10.0, 1), got (%v, %v, %v)",
			result[0].Price, result[0].Volume, result[0].SwapCount)
	}

	if result[1].Price != 2.0 || result[1].Volume != 20.0 || result[1].SwapCount != 1 {
		t.Errorf("Point 1: expected (2.0, 20.0, 1), got (%v, %v, %v)",
			result[1].Price, result[1].Volume, result[1].SwapCount)
	}
}

func TestGeneratePriceTimeseries_SameTimestamp(t *testing.T) {
	// Same timestamp -> aggregate: LAST(price), SUM(volume), COUNT(*)
	swaps := []*domain.Swap{
		{CandidateID: "c1", Timestamp: 1000, Slot: 100, Price: 1.0, AmountOut: 10.0, EventIndex: 0},
		{CandidateID: "c1", Timestamp: 1000, Slot: 101, Price: 1.5, AmountOut: 15.0, EventIndex: 1},
		{CandidateID: "c1", Timestamp: 1000, Slot: 102, Price: 2.0, AmountOut: 20.0, EventIndex: 2},
	}

	result := GeneratePriceTimeseries(swaps)

	if len(result) != 1 {
		t.Fatalf("Expected 1 aggregated point, got %d", len(result))
	}

	// LAST(price) = 2.0, SUM(volume) = 45.0, COUNT = 3
	if result[0].Price != 2.0 {
		t.Errorf("Expected LAST price 2.0, got %v", result[0].Price)
	}
	if result[0].Volume != 45.0 {
		t.Errorf("Expected SUM volume 45.0, got %v", result[0].Volume)
	}
	if result[0].SwapCount != 3 {
		t.Errorf("Expected COUNT 3, got %v", result[0].SwapCount)
	}
}

func TestGenerateLiquidityTimeseries_Basic(t *testing.T) {
	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", Timestamp: 1000, Slot: 100, LiquidityAfter: 1000.0, AmountToken: 100.0, AmountQuote: 10.0},
		{CandidateID: "c1", Timestamp: 2000, Slot: 200, LiquidityAfter: 2000.0, AmountToken: 200.0, AmountQuote: 20.0},
	}

	result := GenerateLiquidityTimeseries(events)

	if len(result) != 2 {
		t.Fatalf("Expected 2 points, got %d", len(result))
	}

	if result[0].Liquidity != 1000.0 {
		t.Errorf("Point 0: expected liquidity 1000.0, got %v", result[0].Liquidity)
	}
	if result[1].Liquidity != 2000.0 {
		t.Errorf("Point 1: expected liquidity 2000.0, got %v", result[1].Liquidity)
	}
}

func TestGenerateLiquidityTimeseries_SameTimestamp(t *testing.T) {
	// Same timestamp -> LAST values
	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", Timestamp: 1000, Slot: 100, LiquidityAfter: 1000.0, AmountToken: 100.0, AmountQuote: 10.0, EventIndex: 0},
		{CandidateID: "c1", Timestamp: 1000, Slot: 101, LiquidityAfter: 1500.0, AmountToken: 150.0, AmountQuote: 15.0, EventIndex: 1},
	}

	result := GenerateLiquidityTimeseries(events)

	if len(result) != 1 {
		t.Fatalf("Expected 1 aggregated point, got %d", len(result))
	}

	// LAST values
	if result[0].Liquidity != 1500.0 {
		t.Errorf("Expected LAST liquidity 1500.0, got %v", result[0].Liquidity)
	}
	if result[0].LiquidityToken != 150.0 {
		t.Errorf("Expected LAST token 150.0, got %v", result[0].LiquidityToken)
	}
}

func TestGenerateVolumeTimeseries_IntervalAlignment(t *testing.T) {
	// Test interval alignment: floor(timestamp_ms / interval_ms) * interval_ms
	swaps := []*domain.Swap{
		{CandidateID: "c1", Timestamp: 60001, AmountOut: 10.0, Side: domain.SwapSideBuy},  // Interval start: 60000
		{CandidateID: "c1", Timestamp: 119999, AmountOut: 20.0, Side: domain.SwapSideBuy}, // Interval start: 60000
		{CandidateID: "c1", Timestamp: 120000, AmountOut: 30.0, Side: domain.SwapSideBuy}, // Interval start: 120000
	}

	result := GenerateVolumeTimeseries(swaps, 60) // 60 second interval

	if len(result) != 2 {
		t.Fatalf("Expected 2 buckets, got %d", len(result))
	}

	// First bucket: 60000
	if result[0].TimestampMs != 60000 {
		t.Errorf("Expected first bucket at 60000, got %d", result[0].TimestampMs)
	}
	if result[0].Volume != 30.0 { // 10 + 20
		t.Errorf("Expected first bucket volume 30.0, got %v", result[0].Volume)
	}
	if result[0].SwapCount != 2 {
		t.Errorf("Expected first bucket count 2, got %d", result[0].SwapCount)
	}

	// Second bucket: 120000
	if result[1].TimestampMs != 120000 {
		t.Errorf("Expected second bucket at 120000, got %d", result[1].TimestampMs)
	}
	if result[1].Volume != 30.0 {
		t.Errorf("Expected second bucket volume 30.0, got %v", result[1].Volume)
	}
}

func TestGenerateVolumeTimeseries_BuySellSeparation(t *testing.T) {
	swaps := []*domain.Swap{
		{CandidateID: "c1", Timestamp: 60000, AmountOut: 10.0, Side: domain.SwapSideBuy},
		{CandidateID: "c1", Timestamp: 60001, AmountOut: 20.0, Side: domain.SwapSideSell},
		{CandidateID: "c1", Timestamp: 60002, AmountOut: 30.0, Side: domain.SwapSideBuy},
	}

	result := GenerateVolumeTimeseries(swaps, 60)

	if len(result) != 1 {
		t.Fatalf("Expected 1 bucket, got %d", len(result))
	}

	if result[0].BuyVolume != 40.0 { // 10 + 30
		t.Errorf("Expected buy volume 40.0, got %v", result[0].BuyVolume)
	}
	if result[0].SellVolume != 20.0 {
		t.Errorf("Expected sell volume 20.0, got %v", result[0].SellVolume)
	}
	if result[0].Volume != 60.0 { // 10 + 20 + 30
		t.Errorf("Expected total volume 60.0, got %v", result[0].Volume)
	}
}

func TestComputeDerivedFeatures_FirstRow(t *testing.T) {
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
	}

	result := ComputeDerivedFeatures(priceTS, nil)

	if len(result) != 1 {
		t.Fatalf("Expected 1 point, got %d", len(result))
	}

	// First row: all derivatives NULL
	if result[0].PriceDelta != nil {
		t.Error("First row: PriceDelta should be NULL")
	}
	if result[0].PriceVelocity != nil {
		t.Error("First row: PriceVelocity should be NULL")
	}
	if result[0].PriceAcceleration != nil {
		t.Error("First row: PriceAcceleration should be NULL")
	}
	if result[0].LastSwapIntervalMs != nil {
		t.Error("First row: LastSwapIntervalMs should be NULL")
	}

	// token_lifetime_ms should be 0 for first row
	if result[0].TokenLifetimeMs != 0 {
		t.Errorf("First row: expected TokenLifetimeMs 0, got %d", result[0].TokenLifetimeMs)
	}
}

func TestComputeDerivedFeatures_SecondRow(t *testing.T) {
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 2000, Price: 2.0},
	}

	result := ComputeDerivedFeatures(priceTS, nil)

	if len(result) != 2 {
		t.Fatalf("Expected 2 points, got %d", len(result))
	}

	// Second row: delta and velocity computed, acceleration NULL
	if result[1].PriceDelta == nil || *result[1].PriceDelta != 1.0 {
		t.Errorf("Second row: expected PriceDelta 1.0, got %v", result[1].PriceDelta)
	}

	// velocity = delta / time_delta = 1.0 / 1000 = 0.001
	if result[1].PriceVelocity == nil || math.Abs(*result[1].PriceVelocity-0.001) > 1e-9 {
		t.Errorf("Second row: expected PriceVelocity 0.001, got %v", result[1].PriceVelocity)
	}

	// Acceleration NULL on second row (need t-1 velocity)
	if result[1].PriceAcceleration != nil {
		t.Error("Second row: PriceAcceleration should be NULL")
	}

	// last_swap_interval_ms = 1000
	if result[1].LastSwapIntervalMs == nil || *result[1].LastSwapIntervalMs != 1000 {
		t.Errorf("Second row: expected LastSwapIntervalMs 1000, got %v", result[1].LastSwapIntervalMs)
	}
}

func TestComputeDerivedFeatures_ThirdRow_Acceleration(t *testing.T) {
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 2000, Price: 2.0},
		{CandidateID: "c1", TimestampMs: 3000, Price: 4.0},
	}

	result := ComputeDerivedFeatures(priceTS, nil)

	if len(result) != 3 {
		t.Fatalf("Expected 3 points, got %d", len(result))
	}

	// Third row: acceleration should be computed
	// velocity[2] = (4.0 - 2.0) / 1000 = 0.002
	// velocity[1] = (2.0 - 1.0) / 1000 = 0.001
	// acceleration = (0.002 - 0.001) / 1000 = 0.000001
	if result[2].PriceAcceleration == nil {
		t.Fatal("Third row: PriceAcceleration should NOT be NULL")
	}
	expectedAccel := 0.000001
	if math.Abs(*result[2].PriceAcceleration-expectedAccel) > 1e-12 {
		t.Errorf("Third row: expected acceleration %v, got %v", expectedAccel, *result[2].PriceAcceleration)
	}
}

func TestComputeDerivedFeatures_TimeDeltaZero(t *testing.T) {
	// Same timestamp -> time_delta = 0 -> velocity/acceleration NULL
	// Note: In reality, GeneratePriceTimeseries would aggregate these.
	// This tests the edge case if somehow same timestamp arrives.
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 1000, Price: 2.0}, // Same timestamp
	}

	result := ComputeDerivedFeatures(priceTS, nil)

	if len(result) != 2 {
		t.Fatalf("Expected 2 points, got %d", len(result))
	}

	// time_delta = 0 -> velocity should be NULL
	if result[1].PriceVelocity != nil {
		t.Error("time_delta=0: PriceVelocity should be NULL")
	}
}

func TestComputeDerivedFeatures_Lifecycle(t *testing.T) {
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 5000, Price: 2.0},
		{CandidateID: "c1", TimestampMs: 10000, Price: 3.0},
	}

	result := ComputeDerivedFeatures(priceTS, nil)

	// token_lifetime_ms = timestamp - MIN(timestamp)
	if result[0].TokenLifetimeMs != 0 {
		t.Errorf("Point 0: expected TokenLifetimeMs 0, got %d", result[0].TokenLifetimeMs)
	}
	if result[1].TokenLifetimeMs != 4000 {
		t.Errorf("Point 1: expected TokenLifetimeMs 4000, got %d", result[1].TokenLifetimeMs)
	}
	if result[2].TokenLifetimeMs != 9000 {
		t.Errorf("Point 2: expected TokenLifetimeMs 9000, got %d", result[2].TokenLifetimeMs)
	}

	// last_swap_interval_ms
	if result[1].LastSwapIntervalMs == nil || *result[1].LastSwapIntervalMs != 4000 {
		t.Errorf("Point 1: expected LastSwapIntervalMs 4000, got %v", result[1].LastSwapIntervalMs)
	}
	if result[2].LastSwapIntervalMs == nil || *result[2].LastSwapIntervalMs != 5000 {
		t.Errorf("Point 2: expected LastSwapIntervalMs 5000, got %v", result[2].LastSwapIntervalMs)
	}
}

func TestComputeDerivedFeatures_UnorderedInput(t *testing.T) {
	// Test that ComputeDerivedFeatures correctly handles unordered input
	// by sorting internally and using MIN(timestamp) for token_lifetime_ms
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 5000, Price: 2.0},  // Out of order
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},  // MIN timestamp
		{CandidateID: "c1", TimestampMs: 10000, Price: 3.0}, // Out of order
	}

	result := ComputeDerivedFeatures(priceTS, nil)

	if len(result) != 3 {
		t.Fatalf("Expected 3 points, got %d", len(result))
	}

	// Results should be sorted by timestamp
	if result[0].TimestampMs != 1000 {
		t.Errorf("Result[0] should be timestamp 1000, got %d", result[0].TimestampMs)
	}
	if result[1].TimestampMs != 5000 {
		t.Errorf("Result[1] should be timestamp 5000, got %d", result[1].TimestampMs)
	}
	if result[2].TimestampMs != 10000 {
		t.Errorf("Result[2] should be timestamp 10000, got %d", result[2].TimestampMs)
	}

	// token_lifetime_ms uses MIN(timestamp) = 1000, not first seen
	if result[0].TokenLifetimeMs != 0 {
		t.Errorf("Point at 1000: expected TokenLifetimeMs 0, got %d", result[0].TokenLifetimeMs)
	}
	if result[1].TokenLifetimeMs != 4000 { // 5000 - 1000
		t.Errorf("Point at 5000: expected TokenLifetimeMs 4000, got %d", result[1].TokenLifetimeMs)
	}
	if result[2].TokenLifetimeMs != 9000 { // 10000 - 1000
		t.Errorf("Point at 10000: expected TokenLifetimeMs 9000, got %d", result[2].TokenLifetimeMs)
	}

	// Deltas should be computed correctly after sorting
	if result[0].PriceDelta != nil {
		t.Error("First point (by timestamp): PriceDelta should be NULL")
	}
	if result[1].PriceDelta == nil || *result[1].PriceDelta != 1.0 { // 2.0 - 1.0
		t.Errorf("Second point: expected PriceDelta 1.0, got %v", result[1].PriceDelta)
	}
	if result[2].PriceDelta == nil || *result[2].PriceDelta != 1.0 { // 3.0 - 2.0
		t.Errorf("Third point: expected PriceDelta 1.0, got %v", result[2].PriceDelta)
	}
}

func TestComputeDerivedFeatures_MissingLiquidity(t *testing.T) {
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 2000, Price: 2.0},
		{CandidateID: "c1", TimestampMs: 3000, Price: 3.0},
	}

	// Liquidity only at timestamp 2000
	liquidityTS := []*domain.LiquidityTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 2000, Liquidity: 1000.0},
	}

	result := ComputeDerivedFeatures(priceTS, liquidityTS)

	// No matching liquidity at 1000 and 3000 -> liquidity features NULL
	if result[0].LiquidityDelta != nil {
		t.Error("Point 0: LiquidityDelta should be NULL (no matching timestamp)")
	}
	if result[2].LiquidityDelta != nil {
		t.Error("Point 2: LiquidityDelta should be NULL (no matching timestamp)")
	}

	// At timestamp 2000, there's liquidity but it's the first, so delta is NULL
	if result[1].LiquidityDelta != nil {
		t.Error("Point 1: LiquidityDelta should be NULL (first liquidity event)")
	}
}

func TestComputeDerivedFeatures_LiquidityDelta(t *testing.T) {
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 2000, Price: 2.0},
	}

	liquidityTS := []*domain.LiquidityTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Liquidity: 1000.0},
		{CandidateID: "c1", TimestampMs: 2000, Liquidity: 1500.0},
	}

	result := ComputeDerivedFeatures(priceTS, liquidityTS)

	// At timestamp 2000: liquidity_delta = 1500 - 1000 = 500
	if result[1].LiquidityDelta == nil || *result[1].LiquidityDelta != 500.0 {
		t.Errorf("Point 1: expected LiquidityDelta 500.0, got %v", result[1].LiquidityDelta)
	}

	// liquidity_velocity = 500 / 1000 = 0.5
	if result[1].LiquidityVelocity == nil || math.Abs(*result[1].LiquidityVelocity-0.5) > 1e-9 {
		t.Errorf("Point 1: expected LiquidityVelocity 0.5, got %v", result[1].LiquidityVelocity)
	}

	// last_liq_event_interval_ms = 1000
	if result[1].LastLiqEventIntervalMs == nil || *result[1].LastLiqEventIntervalMs != 1000 {
		t.Errorf("Point 1: expected LastLiqEventIntervalMs 1000, got %v", result[1].LastLiqEventIntervalMs)
	}
}

func TestRunner_Deterministic(t *testing.T) {
	// Run multiple times and verify same output
	for run := 0; run < 3; run++ {
		swapStore := memory.NewSwapStore()
		liquidityStore := memory.NewLiquidityEventStore()
		priceStore := memory.NewPriceTimeseriesStore()
		liquidityTSStore := memory.NewLiquidityTimeseriesStore()
		volumeStore := memory.NewVolumeTimeseriesStore()
		derivedStore := memory.NewDerivedFeatureStore()
		ctx := context.Background()

		// Insert unordered data
		swaps := []*domain.Swap{
			{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000, Price: 3.0, AmountOut: 30.0, Side: domain.SwapSideBuy},
			{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000, Price: 1.0, AmountOut: 10.0, Side: domain.SwapSideBuy},
			{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000, Price: 2.0, AmountOut: 20.0, Side: domain.SwapSideSell},
		}
		_ = swapStore.InsertBulk(ctx, swaps)

		liquidity := []*domain.LiquidityEvent{
			{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000, LiquidityAfter: 2000.0},
			{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000, LiquidityAfter: 1000.0},
		}
		_ = liquidityStore.InsertBulk(ctx, liquidity)

		runner := NewRunner(swapStore, liquidityStore, priceStore, liquidityTSStore, volumeStore, derivedStore)

		err := runner.NormalizeCandidate(ctx, "c1")
		if err != nil {
			t.Fatalf("Run %d: NormalizeCandidate failed: %v", run, err)
		}

		// Verify price timeseries
		priceTS, _ := priceStore.GetByCandidateID(ctx, "c1")
		if len(priceTS) != 3 {
			t.Fatalf("Run %d: expected 3 price points, got %d", run, len(priceTS))
		}

		// Should be sorted by timestamp
		if priceTS[0].TimestampMs != 1000 || priceTS[1].TimestampMs != 2000 || priceTS[2].TimestampMs != 3000 {
			t.Errorf("Run %d: price timeseries not in order", run)
		}

		// Verify derived features
		derived, _ := derivedStore.GetByCandidateID(ctx, "c1")
		if len(derived) != 3 {
			t.Fatalf("Run %d: expected 3 derived points, got %d", run, len(derived))
		}

		// First point: all derivatives NULL
		if derived[0].PriceDelta != nil {
			t.Errorf("Run %d: first point PriceDelta should be NULL", run)
		}

		// Second point: delta = 1.0
		if derived[1].PriceDelta == nil || *derived[1].PriceDelta != 1.0 {
			t.Errorf("Run %d: second point expected PriceDelta 1.0, got %v", run, derived[1].PriceDelta)
		}
	}
}

func TestRunner_Empty(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liquidityTSStore := memory.NewLiquidityTimeseriesStore()
	volumeStore := memory.NewVolumeTimeseriesStore()
	derivedStore := memory.NewDerivedFeatureStore()
	ctx := context.Background()

	runner := NewRunner(swapStore, liquidityStore, priceStore, liquidityTSStore, volumeStore, derivedStore)

	err := runner.NormalizeCandidate(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Empty candidate should not error: %v", err)
	}

	priceTS, _ := priceStore.GetByCandidateID(ctx, "nonexistent")
	if len(priceTS) != 0 {
		t.Errorf("Expected 0 price points, got %d", len(priceTS))
	}
}

// TestComputeDerivedFeatures_LiquidityEventsBetweenPrices tests that last_liq_event_interval_ms
// correctly tracks ALL liquidity events, not just those matching price timestamps.
// This is critical per NORMALIZATION_SPEC.md section 5.3.
func TestComputeDerivedFeatures_LiquidityEventsBetweenPrices(t *testing.T) {
	// Price events at: 1000, 5000, 10000
	priceTS := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 5000, Price: 2.0},
		{CandidateID: "c1", TimestampMs: 10000, Price: 3.0},
	}

	// Liquidity events at: 500, 2000, 3000, 8000
	// Note: 500 < first price (1000), so no interval for price at 1000
	// Note: 2000 and 3000 are BETWEEN price timestamps 1000 and 5000
	// For price at 5000: prev liquidity event is at 3000 -> interval = 2000
	// For price at 10000: prev liquidity event is at 8000 -> interval = 2000
	liquidityTS := []*domain.LiquidityTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 500, Liquidity: 100.0},
		{CandidateID: "c1", TimestampMs: 2000, Liquidity: 200.0},
		{CandidateID: "c1", TimestampMs: 3000, Liquidity: 300.0},
		{CandidateID: "c1", TimestampMs: 8000, Liquidity: 400.0},
	}

	result := ComputeDerivedFeatures(priceTS, liquidityTS)

	if len(result) != 3 {
		t.Fatalf("Expected 3 points, got %d", len(result))
	}

	// Price at 1000: prev liquidity at 500 -> interval = 500
	if result[0].LastLiqEventIntervalMs == nil {
		t.Error("Point 0 (price at 1000): LastLiqEventIntervalMs should NOT be NULL - there's a liquidity event at 500")
	} else if *result[0].LastLiqEventIntervalMs != 500 {
		t.Errorf("Point 0: expected LastLiqEventIntervalMs 500, got %d", *result[0].LastLiqEventIntervalMs)
	}

	// Price at 5000: prev liquidity at 3000 -> interval = 2000
	if result[1].LastLiqEventIntervalMs == nil {
		t.Error("Point 1 (price at 5000): LastLiqEventIntervalMs should NOT be NULL")
	} else if *result[1].LastLiqEventIntervalMs != 2000 {
		t.Errorf("Point 1: expected LastLiqEventIntervalMs 2000 (5000-3000), got %d", *result[1].LastLiqEventIntervalMs)
	}

	// Price at 10000: prev liquidity at 8000 -> interval = 2000
	if result[2].LastLiqEventIntervalMs == nil {
		t.Error("Point 2 (price at 10000): LastLiqEventIntervalMs should NOT be NULL")
	} else if *result[2].LastLiqEventIntervalMs != 2000 {
		t.Errorf("Point 2: expected LastLiqEventIntervalMs 2000 (10000-8000), got %d", *result[2].LastLiqEventIntervalMs)
	}

	// Verify that LiquidityDelta is still NULL for non-matching timestamps
	// Price at 1000: no liquidity at this exact timestamp
	if result[0].LiquidityDelta != nil {
		t.Error("Point 0: LiquidityDelta should be NULL (no matching liquidity timestamp)")
	}
	// Price at 5000: no liquidity at this exact timestamp
	if result[1].LiquidityDelta != nil {
		t.Error("Point 1: LiquidityDelta should be NULL (no matching liquidity timestamp)")
	}
	// Price at 10000: no liquidity at this exact timestamp
	if result[2].LiquidityDelta != nil {
		t.Error("Point 2: LiquidityDelta should be NULL (no matching liquidity timestamp)")
	}
}

// TestFindPrevLiquidityEventTimestamp tests the binary search helper function
func TestFindPrevLiquidityEventTimestamp(t *testing.T) {
	tests := []struct {
		name       string
		timestamps []int64
		target     int64
		want       *int64
	}{
		{
			name:       "empty list",
			timestamps: nil,
			target:     1000,
			want:       nil,
		},
		{
			name:       "target before all",
			timestamps: []int64{500, 1000, 1500},
			target:     100,
			want:       nil,
		},
		{
			name:       "target equals first",
			timestamps: []int64{500, 1000, 1500},
			target:     500,
			want:       nil, // strictly less than
		},
		{
			name:       "target between first and second",
			timestamps: []int64{500, 1000, 1500},
			target:     750,
			want:       ptrInt64(500),
		},
		{
			name:       "target after all",
			timestamps: []int64{500, 1000, 1500},
			target:     2000,
			want:       ptrInt64(1500),
		},
		{
			name:       "target equals middle",
			timestamps: []int64{500, 1000, 1500},
			target:     1000,
			want:       ptrInt64(500), // strictly less than
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPrevLiquidityEventTimestamp(tt.timestamps, tt.target)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %d", *got)
				}
			} else {
				if got == nil {
					t.Errorf("expected %d, got nil", *tt.want)
				} else if *got != *tt.want {
					t.Errorf("expected %d, got %d", *tt.want, *got)
				}
			}
		})
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
