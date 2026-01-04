package ingestion

import (
	"context"
	"sort"

	"solana-token-lab/internal/storage"
)

// resolveCandidateIDByMint selects a deterministic candidate ID for a mint.
func resolveCandidateIDByMint(ctx context.Context, store storage.CandidateStore, mint string) (string, error) {
	if store == nil || mint == "" {
		return "", nil
	}

	candidates, err := store.GetByMint(ctx, mint)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].DiscoveredAt != candidates[j].DiscoveredAt {
			return candidates[i].DiscoveredAt < candidates[j].DiscoveredAt
		}
		return candidates[i].CandidateID < candidates[j].CandidateID
	})

	return candidates[0].CandidateID, nil
}
