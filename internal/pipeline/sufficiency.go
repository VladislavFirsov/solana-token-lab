package pipeline

import (
	"context"
	"fmt"
	"sort"
	"time"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/storage"
)

// SufficiencyCheck represents one data sufficiency criterion.
type SufficiencyCheck struct {
	Name      string
	Threshold string
	Actual    string
	Pass      bool
}

// SufficiencyResult contains all 6 checks.
type SufficiencyResult struct {
	Checks  []SufficiencyCheck
	AllPass bool
	Errors  []string // data integrity errors
}

// SufficiencyChecker validates data sufficiency before decision.
type SufficiencyChecker struct {
	candidateStore       storage.CandidateStore
	tradeStore           storage.TradeRecordStore
	swapStore            storage.SwapStore
	liquidityStore       storage.LiquidityEventStore
	priceTimeseriesStore storage.PriceTimeseriesStore
	liqTimeseriesStore   storage.LiquidityTimeseriesStore
	replayRunner         *replay.Runner
}

// NewSufficiencyChecker creates a new sufficiency checker.
func NewSufficiencyChecker(
	candidateStore storage.CandidateStore,
	tradeStore storage.TradeRecordStore,
	swapStore storage.SwapStore,
	liquidityStore storage.LiquidityEventStore,
	replayRunner *replay.Runner,
) *SufficiencyChecker {
	return &SufficiencyChecker{
		candidateStore: candidateStore,
		tradeStore:     tradeStore,
		swapStore:      swapStore,
		liquidityStore: liquidityStore,
		replayRunner:   replayRunner,
	}
}

// WithTimeseriesStores adds timeseries stores for coverage check.
func (c *SufficiencyChecker) WithTimeseriesStores(
	priceStore storage.PriceTimeseriesStore,
	liqStore storage.LiquidityTimeseriesStore,
) *SufficiencyChecker {
	c.priceTimeseriesStore = priceStore
	c.liqTimeseriesStore = liqStore
	return c
}

// Check performs all 6 sufficiency checks as defined in DECISION_GATE.md section 1.
func (c *SufficiencyChecker) Check(ctx context.Context) (*SufficiencyResult, error) {
	result := &SufficiencyResult{
		Checks:  make([]SufficiencyCheck, 0, 6),
		AllPass: true,
		Errors:  []string{},
	}

	// Load all candidates (both NEW_TOKEN and ACTIVE_TOKEN)
	newTokenCandidates, err := c.candidateStore.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get NEW_TOKEN candidates: %w", err)
	}

	activeTokenCandidates, err := c.candidateStore.GetBySource(ctx, domain.SourceActiveToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get ACTIVE_TOKEN candidates: %w", err)
	}

	allCandidates := append(newTokenCandidates, activeTokenCandidates...)

	// Check 1: Unique NEW_TOKEN candidates >= 300
	check1 := c.checkUniqueNewTokenCandidates(newTokenCandidates)
	result.Checks = append(result.Checks, check1)
	if !check1.Pass {
		result.AllPass = false
	}

	// Check 2: Discovery uptime >= 7 days (continuous range)
	check2 := c.checkDiscoveryUptime(allCandidates)
	result.Checks = append(result.Checks, check2)
	if !check2.Pass {
		result.AllPass = false
	}

	// Check 3: Backtest data coverage >= 14 days
	check3, err := c.checkBacktestCoverage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check backtest coverage: %w", err)
	}
	result.Checks = append(result.Checks, check3)
	if !check3.Pass {
		result.AllPass = false
	}

	// Check 4: Duplicate candidate_id count == 0
	check4, duplicateErrors := c.checkDuplicateCandidates(allCandidates)
	result.Checks = append(result.Checks, check4)
	if !check4.Pass {
		result.AllPass = false
		result.Errors = append(result.Errors, duplicateErrors...)
	}

	// Check 5: Missing events in evaluation period == 0
	check5, missingErrors := c.checkMissingEvents(ctx, allCandidates)
	result.Checks = append(result.Checks, check5)
	if !check5.Pass {
		result.AllPass = false
		result.Errors = append(result.Errors, missingErrors...)
	}

	// Check 6: Replayable tokens == 100%
	check6, replayErrors := c.checkReplayability(ctx, allCandidates)
	result.Checks = append(result.Checks, check6)
	if !check6.Pass {
		result.AllPass = false
		result.Errors = append(result.Errors, replayErrors...)
	}

	return result, nil
}

// checkUniqueNewTokenCandidates: unique NEW_TOKEN candidates >= 300.
func (c *SufficiencyChecker) checkUniqueNewTokenCandidates(candidates []*domain.TokenCandidate) SufficiencyCheck {
	count := len(candidates)
	return SufficiencyCheck{
		Name:      "Unique NEW_TOKEN candidates",
		Threshold: ">= 300",
		Actual:    fmt.Sprintf("%d", count),
		Pass:      count >= 300,
	}
}

// checkDiscoveryUptime: discovery uptime >= 7 days (continuous range).
// Continuous means at least one candidate per day across the range.
func (c *SufficiencyChecker) checkDiscoveryUptime(candidates []*domain.TokenCandidate) SufficiencyCheck {
	if len(candidates) == 0 {
		return SufficiencyCheck{
			Name:      "Discovery uptime",
			Threshold: ">= 7 days (continuous)",
			Actual:    "0 days",
			Pass:      false,
		}
	}

	// Find min and max discovered_at
	var minTs, maxTs int64 = candidates[0].DiscoveredAt, candidates[0].DiscoveredAt
	for _, cand := range candidates {
		if cand.DiscoveredAt < minTs {
			minTs = cand.DiscoveredAt
		}
		if cand.DiscoveredAt > maxTs {
			maxTs = cand.DiscoveredAt
		}
	}

	// Compute daily coverage
	minTime := time.UnixMilli(minTs).UTC()
	maxTime := time.UnixMilli(maxTs).UTC()

	// Truncate to day
	minDay := time.Date(minTime.Year(), minTime.Month(), minTime.Day(), 0, 0, 0, 0, time.UTC)
	maxDay := time.Date(maxTime.Year(), maxTime.Month(), maxTime.Day(), 0, 0, 0, 0, time.UTC)

	// Range length in days
	rangeDays := int(maxDay.Sub(minDay).Hours()/24) + 1

	if rangeDays < 7 {
		return SufficiencyCheck{
			Name:      "Discovery uptime",
			Threshold: ">= 7 days (continuous)",
			Actual:    fmt.Sprintf("%d days", rangeDays),
			Pass:      false,
		}
	}

	// Check continuity: at least one candidate per day
	daysWithCandidates := make(map[string]bool)
	for _, cand := range candidates {
		t := time.UnixMilli(cand.DiscoveredAt).UTC()
		dayKey := fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
		daysWithCandidates[dayKey] = true
	}

	// Count continuous days from minDay to maxDay
	continuousDays := 0
	for d := minDay; !d.After(maxDay); d = d.AddDate(0, 0, 1) {
		dayKey := fmt.Sprintf("%04d-%02d-%02d", d.Year(), d.Month(), d.Day())
		if daysWithCandidates[dayKey] {
			continuousDays++
		} else {
			// Gap detected - restart count
			continuousDays = 0
		}
		if continuousDays >= 7 {
			return SufficiencyCheck{
				Name:      "Discovery uptime",
				Threshold: ">= 7 days (continuous)",
				Actual:    fmt.Sprintf(">= 7 days (%d total days)", rangeDays),
				Pass:      true,
			}
		}
	}

	return SufficiencyCheck{
		Name:      "Discovery uptime",
		Threshold: ">= 7 days (continuous)",
		Actual:    fmt.Sprintf("%d continuous days (max)", continuousDays),
		Pass:      false,
	}
}

// checkBacktestCoverage: backtest data coverage >= 14 days.
// Per spec: computes time span from price/liquidity timeseries (not events).
// Falls back to swap/liquidity events if timeseries stores not configured.
func (c *SufficiencyChecker) checkBacktestCoverage(ctx context.Context) (SufficiencyCheck, error) {
	var minTime, maxTime int64
	hasData := false

	// Primary: use timeseries stores if available
	if c.priceTimeseriesStore != nil {
		priceMin, priceMax, err := c.priceTimeseriesStore.GetGlobalTimeRange(ctx)
		if err == nil && priceMax > 0 {
			if !hasData {
				minTime = priceMin
				maxTime = priceMax
				hasData = true
			} else {
				if priceMin < minTime {
					minTime = priceMin
				}
				if priceMax > maxTime {
					maxTime = priceMax
				}
			}
		}
	}

	if c.liqTimeseriesStore != nil {
		liqMin, liqMax, err := c.liqTimeseriesStore.GetGlobalTimeRange(ctx)
		if err == nil && liqMax > 0 {
			if !hasData {
				minTime = liqMin
				maxTime = liqMax
				hasData = true
			} else {
				if liqMin < minTime {
					minTime = liqMin
				}
				if liqMax > maxTime {
					maxTime = liqMax
				}
			}
		}
	}

	// Fallback: use swap/liquidity events if timeseries not available
	if !hasData {
		newTokenCandidates, err := c.candidateStore.GetBySource(ctx, domain.SourceNewToken)
		if err != nil {
			return SufficiencyCheck{
				Name:      "Backtest data coverage",
				Threshold: ">= 14 days",
				Actual:    fmt.Sprintf("error loading NEW_TOKEN candidates: %v", err),
				Pass:      false,
			}, nil
		}
		activeTokenCandidates, err := c.candidateStore.GetBySource(ctx, domain.SourceActiveToken)
		if err != nil {
			return SufficiencyCheck{
				Name:      "Backtest data coverage",
				Threshold: ">= 14 days",
				Actual:    fmt.Sprintf("error loading ACTIVE_TOKEN candidates: %v", err),
				Pass:      false,
			}, nil
		}
		allCandidates := append(newTokenCandidates, activeTokenCandidates...)

		for _, candidate := range allCandidates {
			// Check swaps
			if c.swapStore != nil {
				swaps, err := c.swapStore.GetByCandidateID(ctx, candidate.CandidateID)
				if err == nil && len(swaps) > 0 {
					for _, swap := range swaps {
						if !hasData {
							minTime = swap.Timestamp
							maxTime = swap.Timestamp
							hasData = true
						} else {
							if swap.Timestamp < minTime {
								minTime = swap.Timestamp
							}
							if swap.Timestamp > maxTime {
								maxTime = swap.Timestamp
							}
						}
					}
				}
			}

			// Check liquidity events
			if c.liquidityStore != nil {
				liqEvents, err := c.liquidityStore.GetByCandidateID(ctx, candidate.CandidateID)
				if err == nil && len(liqEvents) > 0 {
					for _, liq := range liqEvents {
						if !hasData {
							minTime = liq.Timestamp
							maxTime = liq.Timestamp
							hasData = true
						} else {
							if liq.Timestamp < minTime {
								minTime = liq.Timestamp
							}
							if liq.Timestamp > maxTime {
								maxTime = liq.Timestamp
							}
						}
					}
				}
			}
		}
	}

	if !hasData {
		return SufficiencyCheck{
			Name:      "Backtest data coverage",
			Threshold: ">= 14 days",
			Actual:    "0 days (no timeseries data)",
			Pass:      false,
		}, nil
	}

	// Compute span in days
	durationMs := maxTime - minTime
	durationDays := float64(durationMs) / (24 * 60 * 60 * 1000)

	return SufficiencyCheck{
		Name:      "Backtest data coverage",
		Threshold: ">= 14 days",
		Actual:    fmt.Sprintf("%.1f days", durationDays),
		Pass:      durationDays >= 14,
	}, nil
}

// checkDuplicateCandidates: duplicate candidate_id count == 0.
func (c *SufficiencyChecker) checkDuplicateCandidates(candidates []*domain.TokenCandidate) (SufficiencyCheck, []string) {
	seen := make(map[string]int)
	for _, cand := range candidates {
		seen[cand.CandidateID]++
	}

	duplicateCount := 0
	var errors []string
	// Sort keys for deterministic output
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, id := range keys {
		count := seen[id]
		if count > 1 {
			duplicateCount++
			errors = append(errors, fmt.Sprintf("duplicate candidate_id: %s (count=%d)", id, count))
		}
	}

	return SufficiencyCheck{
		Name:      "Duplicate candidate_id count",
		Threshold: "= 0",
		Actual:    fmt.Sprintf("%d", duplicateCount),
		Pass:      duplicateCount == 0,
	}, errors
}

// checkMissingEvents: missing events in evaluation period == 0.
// For each candidate, require at least one swap AND at least one liquidity event.
func (c *SufficiencyChecker) checkMissingEvents(ctx context.Context, candidates []*domain.TokenCandidate) (SufficiencyCheck, []string) {
	// Validate required stores are configured
	if c.swapStore == nil {
		return SufficiencyCheck{
			Name:      "Missing events count",
			Threshold: "= 0",
			Actual:    "NOT CONFIGURED (swap store required)",
			Pass:      false,
		}, []string{"swap store not configured - cannot verify event coverage"}
	}
	if c.liquidityStore == nil {
		return SufficiencyCheck{
			Name:      "Missing events count",
			Threshold: "= 0",
			Actual:    "NOT CONFIGURED (liquidity store required)",
			Pass:      false,
		}, []string{"liquidity store not configured - cannot verify event coverage"}
	}

	missingSwapsCount := 0
	missingLiquidityCount := 0
	var errors []string

	// Sort candidates by ID for deterministic output
	sortedCandidates := make([]*domain.TokenCandidate, len(candidates))
	copy(sortedCandidates, candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].CandidateID < sortedCandidates[j].CandidateID
	})

	for _, cand := range sortedCandidates {
		// Check swaps
		swaps, err := c.swapStore.GetByCandidateID(ctx, cand.CandidateID)
		if err != nil {
			missingSwapsCount++
			errors = append(errors, fmt.Sprintf("error fetching swaps for candidate %s: %v", cand.CandidateID, err))
		} else if len(swaps) == 0 {
			missingSwapsCount++
			errors = append(errors, fmt.Sprintf("no swaps found for candidate %s", cand.CandidateID))
		}

		// Check liquidity events
		liquidity, err := c.liquidityStore.GetByCandidateID(ctx, cand.CandidateID)
		if err != nil {
			missingLiquidityCount++
			errors = append(errors, fmt.Sprintf("error fetching liquidity events for candidate %s: %v", cand.CandidateID, err))
		} else if len(liquidity) == 0 {
			missingLiquidityCount++
			errors = append(errors, fmt.Sprintf("no liquidity events found for candidate %s", cand.CandidateID))
		}
	}

	totalMissing := missingSwapsCount + missingLiquidityCount
	return SufficiencyCheck{
		Name:      "Missing events count",
		Threshold: "= 0",
		Actual:    fmt.Sprintf("%d missing (%d swaps, %d liquidity)", totalMissing, missingSwapsCount, missingLiquidityCount),
		Pass:      totalMissing == 0,
	}, errors
}

// checkReplayability: replayable tokens == 100%.
// For each candidate, attempt replay with Noop engine.
func (c *SufficiencyChecker) checkReplayability(ctx context.Context, candidates []*domain.TokenCandidate) (SufficiencyCheck, []string) {
	if c.replayRunner == nil {
		return SufficiencyCheck{
			Name:      "Replayable tokens",
			Threshold: "= 100%",
			Actual:    "NOT CONFIGURED (replay runner required)",
			Pass:      false, // Fail if no replay runner - cannot verify replayability
		}, []string{"replay runner not configured - cannot verify replayability requirement"}
	}

	totalCount := len(candidates)
	if totalCount == 0 {
		return SufficiencyCheck{
			Name:      "Replayable tokens",
			Threshold: "= 100%",
			Actual:    "0/0 (no candidates)",
			Pass:      true,
		}, nil
	}

	failedCount := 0
	var errors []string

	// Sort candidates by ID for deterministic output
	sortedCandidates := make([]*domain.TokenCandidate, len(candidates))
	copy(sortedCandidates, candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		return sortedCandidates[i].CandidateID < sortedCandidates[j].CandidateID
	})

	for _, cand := range sortedCandidates {
		err := c.replayRunner.RunAll(ctx, cand.CandidateID, &noopEngine{})
		if err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("replay failed for candidate %s: %v", cand.CandidateID, err))
		}
	}

	replayableCount := totalCount - failedCount
	pct := float64(replayableCount) / float64(totalCount) * 100

	return SufficiencyCheck{
		Name:      "Replayable tokens",
		Threshold: "= 100%",
		Actual:    fmt.Sprintf("%.1f%% (%d/%d)", pct, replayableCount, totalCount),
		Pass:      failedCount == 0,
	}, errors
}

// noopEngine is a ReplayEngine that does nothing - used for replayability check.
type noopEngine struct{}

func (e *noopEngine) OnEvent(ctx context.Context, event *replay.Event) error {
	return nil
}
