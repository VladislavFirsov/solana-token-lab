package strategy

import (
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
)

func TestFromConfig_TimeExit(t *testing.T) {
	holdDuration := int64(60000)
	cfg := domain.StrategyConfig{
		StrategyType:   domain.StrategyTypeTimeExit,
		EntryEventType: "NEW_TOKEN",
		HoldDurationMs: &holdDuration,
	}

	s, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig failed: %v", err)
	}

	te, ok := s.(*TimeExitStrategy)
	if !ok {
		t.Fatalf("expected *TimeExitStrategy, got %T", s)
	}

	if te.EntryEventType != "NEW_TOKEN" {
		t.Errorf("expected NEW_TOKEN, got %s", te.EntryEventType)
	}
	if te.HoldDurationMs != 60000 {
		t.Errorf("expected 60000, got %d", te.HoldDurationMs)
	}
}

func TestFromConfig_TrailingStop(t *testing.T) {
	trailPct := 0.10
	initialStopPct := 0.10
	maxHoldMs := int64(3600000)
	cfg := domain.StrategyConfig{
		StrategyType:      domain.StrategyTypeTrailingStop,
		EntryEventType:    "ACTIVE_TOKEN",
		TrailPct:          &trailPct,
		InitialStopPct:    &initialStopPct,
		MaxHoldDurationMs: &maxHoldMs,
	}

	s, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig failed: %v", err)
	}

	ts, ok := s.(*TrailingStopStrategy)
	if !ok {
		t.Fatalf("expected *TrailingStopStrategy, got %T", s)
	}

	if ts.EntryEventType != "ACTIVE_TOKEN" {
		t.Errorf("expected ACTIVE_TOKEN, got %s", ts.EntryEventType)
	}
	if ts.TrailPct != 0.10 {
		t.Errorf("expected 0.10, got %f", ts.TrailPct)
	}
	if ts.InitialStopPct != 0.10 {
		t.Errorf("expected 0.10, got %f", ts.InitialStopPct)
	}
	if ts.MaxHoldDurationMs != 3600000 {
		t.Errorf("expected 3600000, got %d", ts.MaxHoldDurationMs)
	}
}

func TestFromConfig_LiquidityGuard(t *testing.T) {
	liquidityDropPct := 0.30
	maxHoldMs := int64(1800000)
	cfg := domain.StrategyConfig{
		StrategyType:      domain.StrategyTypeLiquidityGuard,
		EntryEventType:    "NEW_TOKEN",
		LiquidityDropPct:  &liquidityDropPct,
		MaxHoldDurationMs: &maxHoldMs,
	}

	s, err := FromConfig(cfg)
	if err != nil {
		t.Fatalf("FromConfig failed: %v", err)
	}

	lg, ok := s.(*LiquidityGuardStrategy)
	if !ok {
		t.Fatalf("expected *LiquidityGuardStrategy, got %T", s)
	}

	if lg.EntryEventType != "NEW_TOKEN" {
		t.Errorf("expected NEW_TOKEN, got %s", lg.EntryEventType)
	}
	if lg.LiquidityDropPct != 0.30 {
		t.Errorf("expected 0.30, got %f", lg.LiquidityDropPct)
	}
	if lg.MaxHoldDurationMs != 1800000 {
		t.Errorf("expected 1800000, got %d", lg.MaxHoldDurationMs)
	}
}

func TestFromConfig_MissingParams(t *testing.T) {
	tests := []struct {
		name        string
		cfg         domain.StrategyConfig
		expectedErr error
	}{
		{
			name: "TIME_EXIT missing HoldDurationMs",
			cfg: domain.StrategyConfig{
				StrategyType:   domain.StrategyTypeTimeExit,
				EntryEventType: "NEW_TOKEN",
			},
			expectedErr: ErrMissingHoldDuration,
		},
		{
			name: "TRAILING_STOP missing TrailPct",
			cfg: domain.StrategyConfig{
				StrategyType:   domain.StrategyTypeTrailingStop,
				EntryEventType: "NEW_TOKEN",
			},
			expectedErr: ErrMissingTrailPct,
		},
		{
			name: "TRAILING_STOP missing InitialStopPct",
			cfg: domain.StrategyConfig{
				StrategyType:   domain.StrategyTypeTrailingStop,
				EntryEventType: "NEW_TOKEN",
				TrailPct:       ptrFloat(0.10),
			},
			expectedErr: ErrMissingInitialStopPct,
		},
		{
			name: "TRAILING_STOP missing MaxHoldDurationMs",
			cfg: domain.StrategyConfig{
				StrategyType:   domain.StrategyTypeTrailingStop,
				EntryEventType: "NEW_TOKEN",
				TrailPct:       ptrFloat(0.10),
				InitialStopPct: ptrFloat(0.10),
			},
			expectedErr: ErrMissingMaxHoldDuration,
		},
		{
			name: "LIQUIDITY_GUARD missing LiquidityDropPct",
			cfg: domain.StrategyConfig{
				StrategyType:   domain.StrategyTypeLiquidityGuard,
				EntryEventType: "NEW_TOKEN",
			},
			expectedErr: ErrMissingLiquidityDropPct,
		},
		{
			name: "LIQUIDITY_GUARD missing MaxHoldDurationMs",
			cfg: domain.StrategyConfig{
				StrategyType:     domain.StrategyTypeLiquidityGuard,
				EntryEventType:   "NEW_TOKEN",
				LiquidityDropPct: ptrFloat(0.30),
			},
			expectedErr: ErrMissingMaxHoldDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FromConfig(tt.cfg)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestFromConfig_UnknownType(t *testing.T) {
	cfg := domain.StrategyConfig{
		StrategyType:   "UNKNOWN",
		EntryEventType: "NEW_TOKEN",
	}

	_, err := FromConfig(cfg)
	if !errors.Is(err, ErrUnknownStrategyType) {
		t.Errorf("expected ErrUnknownStrategyType, got %v", err)
	}
}

// Helper functions
func ptrFloat(f float64) *float64 {
	return &f
}
