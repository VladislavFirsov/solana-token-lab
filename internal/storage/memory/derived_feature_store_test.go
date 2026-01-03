package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestDerivedFeatureStore_InsertBulkAndGet(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	priceDelta := 0.1
	priceVelocity := 0.01

	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "c1", TimestampMs: 1000, TokenLifetimeMs: 1000},
		{CandidateID: "c1", TimestampMs: 2000, TokenLifetimeMs: 2000, PriceDelta: &priceDelta, PriceVelocity: &priceVelocity},
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

	// Second point should have derived values
	if result[1].PriceDelta == nil || *result[1].PriceDelta != 0.1 {
		t.Error("PriceDelta should be 0.1")
	}
}

func TestDerivedFeatureStore_DuplicateKey(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "c1", TimestampMs: 1000, TokenLifetimeMs: 1000},
	}

	if err := store.InsertBulk(ctx, points); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.InsertBulk(ctx, points)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestDerivedFeatureStore_IntraBatchDuplicate(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "c1", TimestampMs: 1000, TokenLifetimeMs: 1000},
		{CandidateID: "c1", TimestampMs: 1000, TokenLifetimeMs: 1001}, // duplicate key
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

func TestDerivedFeatureStore_GetByTimeRange(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "c1", TimestampMs: 1000, TokenLifetimeMs: 1000},
		{CandidateID: "c1", TimestampMs: 2000, TokenLifetimeMs: 2000},
		{CandidateID: "c1", TimestampMs: 3000, TokenLifetimeMs: 3000},
		{CandidateID: "c2", TimestampMs: 2500, TokenLifetimeMs: 2500}, // different candidate
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

func TestDerivedFeatureStore_OrderByTimestamp(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "c1", TimestampMs: 3000, TokenLifetimeMs: 3000},
		{CandidateID: "c1", TimestampMs: 1000, TokenLifetimeMs: 1000},
		{CandidateID: "c1", TimestampMs: 2000, TokenLifetimeMs: 2000},
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

func TestDerivedFeatureStore_InvalidInput(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	err := store.InsertBulk(ctx, []*domain.DerivedFeaturePoint{nil})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil point, got %v", err)
	}

	err = store.InsertBulk(ctx, []*domain.DerivedFeaturePoint{{CandidateID: ""}})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty CandidateID, got %v", err)
	}
}

func TestDerivedFeatureStore_EmptyBulk(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	err := store.InsertBulk(ctx, []*domain.DerivedFeaturePoint{})
	if err != nil {
		t.Errorf("Empty bulk should succeed, got %v", err)
	}
}

func TestDerivedFeatureStore_NullableFields(t *testing.T) {
	store := NewDerivedFeatureStore()
	ctx := context.Background()

	// First point - no derived values (first row)
	point := &domain.DerivedFeaturePoint{
		CandidateID:     "c1",
		TimestampMs:     1000,
		TokenLifetimeMs: 1000,
		// All nullable fields are nil
	}

	if err := store.InsertBulk(ctx, []*domain.DerivedFeaturePoint{point}); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if result[0].PriceDelta != nil {
		t.Error("PriceDelta should be nil for first row")
	}
	if result[0].PriceVelocity != nil {
		t.Error("PriceVelocity should be nil for first row")
	}
}
