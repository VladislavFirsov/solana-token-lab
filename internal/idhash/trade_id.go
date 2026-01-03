package idhash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ComputeTradeID computes a deterministic trade_id using SHA256.
// Formula: SHA256(candidate_id|strategy_id|scenario_id|entry_signal_time)
// Returns hex-encoded hash (64 characters).
func ComputeTradeID(
	candidateID string,
	strategyID string,
	scenarioID string,
	entrySignalTime int64,
) string {
	data := fmt.Sprintf("%s|%s|%s|%d",
		candidateID,
		strategyID,
		scenarioID,
		entrySignalTime,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
