package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestPriceTimeseriesStore_InsertBulkAndGet(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Slot: 100, Price: 1.0, Volume: 100.0, SwapCount: 5},
		{CandidateID: "c1", TimestampMs: 2000, Slot: 200, Price: 1.1, Volume: 150.0, SwapCount: 7},
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

func TestPriceTimeseriesStore_DuplicateKey(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
	}

	if err := store.InsertBulk(ctx, points); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Insert duplicate
	err := store.InsertBulk(ctx, points)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestPriceTimeseriesStore_IntraBatchDuplicate(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.1}, // duplicate key
	}

	err := store.InsertBulk(ctx, points)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey for intra-batch duplicate, got %v", err)
	}

	// Verify nothing was inserted
	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 0 {
		t.Errorf("Expected 0 points (rollback), got %d", len(result))
	}
}

func TestPriceTimeseriesStore_GetByTimeRange(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 2000, Price: 1.1},
		{CandidateID: "c1", TimestampMs: 3000, Price: 1.2},
		{CandidateID: "c2", TimestampMs: 2500, Price: 2.0}, // different candidate
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

func TestPriceTimeseriesStore_OrderByTimestamp(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 3000, Price: 1.2},
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0},
		{CandidateID: "c1", TimestampMs: 2000, Price: 1.1},
	}

	if err := store.InsertBulk(ctx, points); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")

	for i := 1; i < len(result); i++ {
		if result[i].TimestampMs < result[i-1].TimestampMs {
			t.Errorf("Results not ordered: %d < %d", result[i].TimestampMs, result[i-1].TimestampMs)
		}
	}
}

func TestPriceTimeseriesStore_InvalidInput(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	err := store.InsertBulk(ctx, []*domain.PriceTimeseriesPoint{nil})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil point, got %v", err)
	}

	err = store.InsertBulk(ctx, []*domain.PriceTimeseriesPoint{{CandidateID: ""}})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty CandidateID, got %v", err)
	}
}

func TestPriceTimeseriesStore_EmptyBulk(t *testing.T) {
	store := NewPriceTimeseriesStore()
	ctx := context.Background()

	err := store.InsertBulk(ctx, []*domain.PriceTimeseriesPoint{})
	if err != nil {
		t.Errorf("Empty bulk should succeed, got %v", err)
	}
}
