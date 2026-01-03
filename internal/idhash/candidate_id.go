package idhash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"solana-token-lab/internal/domain"
)

// ComputeCandidateID computes a deterministic candidate_id using SHA256.
// Formula: SHA256(mint|pool|source|tx_signature|event_index|slot)
// Returns hex-encoded hash (64 characters).
func ComputeCandidateID(
	mint string,
	pool *string,
	source domain.Source,
	txSignature string,
	eventIndex int,
	slot int64,
) string {
	poolStr := ""
	if pool != nil {
		poolStr = *pool
	}

	data := fmt.Sprintf("%s|%s|%s|%s|%d|%d",
		mint,
		poolStr,
		string(source),
		txSignature,
		eventIndex,
		slot,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
