package normalization

import (
	"sort"

	"solana-token-lab/internal/domain"
)

// GenerateVolumeTimeseries aggregates swaps into volume buckets by interval.
// Swaps must be pre-sorted by (slot, tx_signature, event_index).
//
// Interval alignment: floor(timestamp_ms / interval_ms) * interval_ms
// Aggregation per (candidate_id, interval_start):
//   - volume = SUM(amount_out)
//   - swap_count = COUNT(*)
//   - buy_volume = SUM(amount_out) WHERE side = 'buy'
//   - sell_volume = SUM(amount_out) WHERE side = 'sell'
func GenerateVolumeTimeseries(swaps []*domain.Swap, intervalSeconds int) []*domain.VolumeTimeseriesPoint {
	if len(swaps) == 0 || intervalSeconds <= 0 {
		return nil
	}

	intervalMs := int64(intervalSeconds) * 1000

	// Map: candidateID -> intervalStart -> point
	buckets := make(map[string]map[int64]*domain.VolumeTimeseriesPoint)

	for _, s := range swaps {
		intervalStart := (s.Timestamp / intervalMs) * intervalMs

		candidateBuckets, ok := buckets[s.CandidateID]
		if !ok {
			candidateBuckets = make(map[int64]*domain.VolumeTimeseriesPoint)
			buckets[s.CandidateID] = candidateBuckets
		}

		point, ok := candidateBuckets[intervalStart]
		if !ok {
			point = &domain.VolumeTimeseriesPoint{
				CandidateID:     s.CandidateID,
				TimestampMs:     intervalStart,
				IntervalSeconds: intervalSeconds,
			}
			candidateBuckets[intervalStart] = point
		}

		point.Volume += s.AmountOut
		point.SwapCount++

		if s.Side == domain.SwapSideBuy {
			point.BuyVolume += s.AmountOut
		} else if s.Side == domain.SwapSideSell {
			point.SellVolume += s.AmountOut
		}
	}

	// Flatten and sort by (candidateID, intervalStart)
	var result []*domain.VolumeTimeseriesPoint
	for _, candidateBuckets := range buckets {
		for _, point := range candidateBuckets {
			result = append(result, point)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].CandidateID != result[j].CandidateID {
			return result[i].CandidateID < result[j].CandidateID
		}
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result
}

// GenerateAllVolumeTimeseries generates volume timeseries for all supported intervals.
func GenerateAllVolumeTimeseries(swaps []*domain.Swap) []*domain.VolumeTimeseriesPoint {
	var result []*domain.VolumeTimeseriesPoint

	intervals := []int{
		domain.VolumeInterval1Min,
		domain.VolumeInterval5Min,
		domain.VolumeInterval1Hour,
	}

	for _, interval := range intervals {
		points := GenerateVolumeTimeseries(swaps, interval)
		result = append(result, points...)
	}

	return result
}
