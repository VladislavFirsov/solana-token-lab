package strategy

import (
	"errors"

	"solana-token-lab/internal/domain"
)

// Factory errors
var (
	ErrUnknownStrategyType     = errors.New("unknown strategy type")
	ErrMissingHoldDuration     = errors.New("TIME_EXIT requires HoldDurationMs")
	ErrMissingTrailPct         = errors.New("TRAILING_STOP requires TrailPct")
	ErrMissingInitialStopPct   = errors.New("TRAILING_STOP requires InitialStopPct")
	ErrMissingMaxHoldDuration  = errors.New("TRAILING_STOP/LIQUIDITY_GUARD requires MaxHoldDurationMs")
	ErrMissingLiquidityDropPct = errors.New("LIQUIDITY_GUARD requires LiquidityDropPct")
)

// FromConfig creates a Strategy from domain.StrategyConfig.
// Validates required parameters per strategy type.
// Returns clear errors for missing/invalid params.
func FromConfig(cfg domain.StrategyConfig) (Strategy, error) {
	switch cfg.StrategyType {
	case domain.StrategyTypeTimeExit:
		return fromTimeExitConfig(cfg)
	case domain.StrategyTypeTrailingStop:
		return fromTrailingStopConfig(cfg)
	case domain.StrategyTypeLiquidityGuard:
		return fromLiquidityGuardConfig(cfg)
	default:
		return nil, ErrUnknownStrategyType
	}
}

// fromTimeExitConfig creates TimeExitStrategy from config.
func fromTimeExitConfig(cfg domain.StrategyConfig) (*TimeExitStrategy, error) {
	if cfg.HoldDurationMs == nil {
		return nil, ErrMissingHoldDuration
	}

	return NewTimeExitStrategy(cfg.EntryEventType, *cfg.HoldDurationMs), nil
}

// fromTrailingStopConfig creates TrailingStopStrategy from config.
func fromTrailingStopConfig(cfg domain.StrategyConfig) (*TrailingStopStrategy, error) {
	if cfg.TrailPct == nil {
		return nil, ErrMissingTrailPct
	}
	if cfg.InitialStopPct == nil {
		return nil, ErrMissingInitialStopPct
	}
	if cfg.MaxHoldDurationMs == nil {
		return nil, ErrMissingMaxHoldDuration
	}

	return NewTrailingStopStrategy(
		cfg.EntryEventType,
		*cfg.TrailPct,
		*cfg.InitialStopPct,
		*cfg.MaxHoldDurationMs,
	), nil
}

// fromLiquidityGuardConfig creates LiquidityGuardStrategy from config.
func fromLiquidityGuardConfig(cfg domain.StrategyConfig) (*LiquidityGuardStrategy, error) {
	if cfg.LiquidityDropPct == nil {
		return nil, ErrMissingLiquidityDropPct
	}
	if cfg.MaxHoldDurationMs == nil {
		return nil, ErrMissingMaxHoldDuration
	}

	return NewLiquidityGuardStrategy(
		cfg.EntryEventType,
		*cfg.LiquidityDropPct,
		*cfg.MaxHoldDurationMs,
	), nil
}
