package normalization

import (
	"solana-token-lab/internal/domain"
)

// GenerateLiquidityTimeseries transforms sorted liquidity events into liquidity_timeseries points.
// Events must be pre-sorted by (slot, tx_signature, event_index).
//
// Aggregation for same (candidate_id, timestamp_ms):
//   - liquidity = LAST(liquidity_after) by event order
//   - liquidity_token = LAST(amount_token)
//   - liquidity_quote = LAST(amount_quote)
func GenerateLiquidityTimeseries(events []*domain.LiquidityEvent) []*domain.LiquidityTimeseriesPoint {
	if len(events) == 0 {
		return nil
	}

	var result []*domain.LiquidityTimeseriesPoint
	var current *domain.LiquidityTimeseriesPoint

	for _, e := range events {
		if current == nil || current.CandidateID != e.CandidateID || current.TimestampMs != e.Timestamp {
			// Start new point
			if current != nil {
				result = append(result, current)
			}
			current = &domain.LiquidityTimeseriesPoint{
				CandidateID:    e.CandidateID,
				TimestampMs:    e.Timestamp,
				Slot:           e.Slot,
				Liquidity:      e.LiquidityAfter,
				LiquidityToken: e.AmountToken,
				LiquidityQuote: e.AmountQuote,
			}
		} else {
			// Aggregate into current point (LAST values)
			current.Slot = e.Slot
			current.Liquidity = e.LiquidityAfter
			current.LiquidityToken = e.AmountToken
			current.LiquidityQuote = e.AmountQuote
		}
	}

	// Don't forget last point
	if current != nil {
		result = append(result, current)
	}

	return result
}
