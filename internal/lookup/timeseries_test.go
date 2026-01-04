package lookup

import (
	"testing"

	"solana-token-lab/internal/domain"
)

func TestPriceAt_EmptySlice(t *testing.T) {
	_, err := PriceAt(1000, nil)
	if err != ErrNoPriceData {
		t.Errorf("expected ErrNoPriceData, got %v", err)
	}

	_, err = PriceAt(1000, []*domain.PriceTimeseriesPoint{})
	if err != ErrNoPriceData {
		t.Errorf("expected ErrNoPriceData, got %v", err)
	}
}

func TestPriceAt_ExactMatch(t *testing.T) {
	prices := []*domain.PriceTimeseriesPoint{
		{TimestampMs: 1000, Price: 1.0},
		{TimestampMs: 2000, Price: 2.0},
		{TimestampMs: 3000, Price: 3.0},
	}

	price, err := PriceAt(2000, prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 2.0 {
		t.Errorf("expected 2.0, got %f", price)
	}
}

func TestPriceAt_BeforeTarget(t *testing.T) {
	prices := []*domain.PriceTimeseriesPoint{
		{TimestampMs: 1000, Price: 1.0},
		{TimestampMs: 2000, Price: 2.0},
		{TimestampMs: 3000, Price: 3.0},
	}

	// Target 2500 should return price at 2000
	price, err := PriceAt(2500, prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 2.0 {
		t.Errorf("expected 2.0, got %f", price)
	}
}

func TestPriceAt_BeforeFirst(t *testing.T) {
	prices := []*domain.PriceTimeseriesPoint{
		{TimestampMs: 1000, Price: 1.0},
		{TimestampMs: 2000, Price: 2.0},
		{TimestampMs: 3000, Price: 3.0},
	}

	// Target 500 should return first price (1.0)
	price, err := PriceAt(500, prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 1.0 {
		t.Errorf("expected 1.0, got %f", price)
	}
}

func TestPriceAt_AfterLast(t *testing.T) {
	prices := []*domain.PriceTimeseriesPoint{
		{TimestampMs: 1000, Price: 1.0},
		{TimestampMs: 2000, Price: 2.0},
		{TimestampMs: 3000, Price: 3.0},
	}

	// Target 5000 should return last price (3.0)
	price, err := PriceAt(5000, prices)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 3.0 {
		t.Errorf("expected 3.0, got %f", price)
	}
}

func TestLiquidityAt_EmptySlice(t *testing.T) {
	_, err := LiquidityAt(1000, nil)
	if err != ErrNoLiquidityData {
		t.Errorf("expected ErrNoLiquidityData, got %v", err)
	}

	_, err = LiquidityAt(1000, []*domain.LiquidityTimeseriesPoint{})
	if err != ErrNoLiquidityData {
		t.Errorf("expected ErrNoLiquidityData, got %v", err)
	}
}

func TestLiquidityAt_ExactMatch(t *testing.T) {
	liq := []*domain.LiquidityTimeseriesPoint{
		{TimestampMs: 1000, Liquidity: 100},
		{TimestampMs: 2000, Liquidity: 200},
		{TimestampMs: 3000, Liquidity: 300},
	}

	l, err := LiquidityAt(2000, liq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil || *l != 200 {
		t.Errorf("expected 200, got %v", l)
	}
}

func TestLiquidityAt_BeforeFirst(t *testing.T) {
	liq := []*domain.LiquidityTimeseriesPoint{
		{TimestampMs: 1000, Liquidity: 100},
		{TimestampMs: 2000, Liquidity: 200},
	}

	// Target 500 is before first event, should return nil (valid case)
	l, err := LiquidityAt(500, liq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l != nil {
		t.Errorf("expected nil, got %v", l)
	}
}

func TestLiquidityAt_AfterLast(t *testing.T) {
	liq := []*domain.LiquidityTimeseriesPoint{
		{TimestampMs: 1000, Liquidity: 100},
		{TimestampMs: 2000, Liquidity: 200},
	}

	// Target 3000 should return last liquidity
	l, err := LiquidityAt(3000, liq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l == nil || *l != 200 {
		t.Errorf("expected 200, got %v", l)
	}
}
