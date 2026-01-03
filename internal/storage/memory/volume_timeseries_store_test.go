package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestVolumeTimeseriesStore_InsertBulkAndGet(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 100.0, SwapCount: 5, BuyVolume: 60.0, SellVolume: 40.0},
		{CandidateID: "c1", TimestampMs: 2000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 150.0, SwapCount: 7, BuyVolume: 80.0, SellVolume: 70.0},
	}

	err := store.InsertBulk(ctx, points)
	if err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetByCandidateID(ctx, "c1")
	if err != nil {
		t.Fatalf("GetByCandidateID failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 points, got %d", len(result))
	}
}

func TestVolumeTimeseriesStore_DuplicateKey(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 100.0},
	}

	if err := store.InsertBulk(ctx, points); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.InsertBulk(ctx, points)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestVolumeTimeseriesStore_SameTimestampDifferentInterval(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	// Same candidate and timestamp but different intervals - should be allowed
	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 100.0},
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval5Min, Volume: 500.0},
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Hour, Volume: 3000.0},
	}

	err := store.InsertBulk(ctx, points)
	if err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 3 {
		t.Errorf("Expected 3 points (different intervals), got %d", len(result))
	}
}

func TestVolumeTimeseriesStore_IntraBatchDuplicate(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 100.0},
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 150.0}, // duplicate key
	}

	err := store.InsertBulk(ctx, points)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey for intra-batch duplicate, got %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 0 {
		t.Errorf("Expected 0 points (rollback), got %d", len(result))
	}
}

func TestVolumeTimeseriesStore_GetByTimeRange(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 100.0},
		{CandidateID: "c1", TimestampMs: 2000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 150.0},
		{CandidateID: "c1", TimestampMs: 3000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 200.0},
		{CandidateID: "c2", TimestampMs: 2500, IntervalSeconds: domain.VolumeInterval1Min, Volume: 300.0}, // different candidate
	}

	if err := store.InsertBulk(ctx, points); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetByTimeRange(ctx, "c1", 1500, 2500)
	if err != nil {
		t.Fatalf("GetByTimeRange failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 point in range, got %d", len(result))
	}

	if result[0].TimestampMs != 2000 {
		t.Errorf("Expected timestamp 2000, got %d", result[0].TimestampMs)
	}
}

func TestVolumeTimeseriesStore_OrderByTimestampThenInterval(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 2000, IntervalSeconds: domain.VolumeInterval5Min, Volume: 500.0},
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Hour, Volume: 3000.0},
		{CandidateID: "c1", TimestampMs: 1000, IntervalSeconds: domain.VolumeInterval1Min, Volume: 100.0},
	}

	if err := store.InsertBulk(ctx, points); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")

	// Should be ordered by timestamp first, then by interval
	if result[0].TimestampMs != 1000 || result[0].IntervalSeconds != domain.VolumeInterval1Min {
		t.Errorf("First should be ts=1000, interval=60, got ts=%d, interval=%d", result[0].TimestampMs, result[0].IntervalSeconds)
	}
	if result[1].TimestampMs != 1000 || result[1].IntervalSeconds != domain.VolumeInterval1Hour {
		t.Errorf("Second should be ts=1000, interval=3600, got ts=%d, interval=%d", result[1].TimestampMs, result[1].IntervalSeconds)
	}
	if result[2].TimestampMs != 2000 {
		t.Errorf("Third should be ts=2000, got ts=%d", result[2].TimestampMs)
	}
}

func TestVolumeTimeseriesStore_InvalidInput(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	err := store.InsertBulk(ctx, []*domain.VolumeTimeseriesPoint{nil})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil point, got %v", err)
	}

	err = store.InsertBulk(ctx, []*domain.VolumeTimeseriesPoint{{CandidateID: ""}})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty CandidateID, got %v", err)
	}
}

func TestVolumeTimeseriesStore_EmptyBulk(t *testing.T) {
	store := NewVolumeTimeseriesStore()
	ctx := context.Background()

	err := store.InsertBulk(ctx, []*domain.VolumeTimeseriesPoint{})
	if err != nil {
		t.Errorf("Empty bulk should succeed, got %v", err)
	}
}
