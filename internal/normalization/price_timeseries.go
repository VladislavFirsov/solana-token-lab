package normalization

import (
	"solana-token-lab/internal/domain"
)

// GeneratePriceTimeseries transforms sorted swaps into price_timeseries points.
// Swaps must be pre-sorted by (slot, tx_signature, event_index).
//
// Aggregation for same (candidate_id, timestamp_ms):
//   - price = LAST(price) by event order
//   - volume = SUM(amount_out)
//   - swap_count = COUNT(*)
func GeneratePriceTimeseries(swaps []*domain.Swap) []*domain.PriceTimeseriesPoint {
	if len(swaps) == 0 {
		return nil
	}

	var result []*domain.PriceTimeseriesPoint
	var current *domain.PriceTimeseriesPoint

	for _, s := range swaps {
		if current == nil || current.CandidateID != s.CandidateID || current.TimestampMs != s.Timestamp {
			// Start new point
			if current != nil {
				result = append(result, current)
			}
			current = &domain.PriceTimeseriesPoint{
				CandidateID: s.CandidateID,
				TimestampMs: s.Timestamp,
				Slot:        s.Slot,
				Price:       s.Price,
				Volume:      s.AmountOut,
				SwapCount:   1,
			}
		} else {
			// Aggregate into current point
			current.Price = s.Price          // LAST(price)
			current.Slot = s.Slot            // LAST(slot)
			current.Volume += s.AmountOut    // SUM(amount_out)
			current.SwapCount++              // COUNT(*)
		}
	}

	// Don't forget last point
	if current != nil {
		result = append(result, current)
	}

	return result
}
