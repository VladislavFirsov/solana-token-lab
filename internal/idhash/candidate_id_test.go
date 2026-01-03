package idhash

import (
	"testing"

	"solana-token-lab/internal/domain"
)

func TestComputeCandidateID(t *testing.T) {
	tests := []struct {
		name        string
		mint        string
		pool        *string
		source      domain.Source
		txSignature string
		eventIndex  int
		slot        int64
		wantLen     int // hash length should be 64
	}{
		{
			name:        "NEW_TOKEN with pool",
			mint:        "TokenMint123ABC",
			pool:        strPtr("PoolAddr456DEF"),
			source:      domain.SourceNewToken,
			txSignature: "TxSig789GHI",
			eventIndex:  0,
			slot:        12345678,
			wantLen:     64,
		},
		{
			name:        "NEW_TOKEN without pool",
			mint:        "TokenMint123ABC",
			pool:        nil,
			source:      domain.SourceNewToken,
			txSignature: "TxSig789GHI",
			eventIndex:  0,
			slot:        12345678,
			wantLen:     64,
		},
		{
			name:        "ACTIVE_TOKEN with pool",
			mint:        "AnotherMint999",
			pool:        strPtr("SomePool111"),
			source:      domain.SourceActiveToken,
			txSignature: "DifferentTx222",
			eventIndex:  5,
			slot:        99999999,
			wantLen:     64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeCandidateID(tt.mint, tt.pool, tt.source, tt.txSignature, tt.eventIndex, tt.slot)

			if len(got) != tt.wantLen {
				t.Errorf("ComputeCandidateID() length = %d, want %d", len(got), tt.wantLen)
			}

			// Verify determinism: same inputs should produce same output
			got2 := ComputeCandidateID(tt.mint, tt.pool, tt.source, tt.txSignature, tt.eventIndex, tt.slot)
			if got != got2 {
				t.Errorf("ComputeCandidateID() not deterministic: %s != %s", got, got2)
			}
		})
	}
}

func TestComputeCandidateID_Determinism(t *testing.T) {
	mint := "TestMint"
	pool := strPtr("TestPool")
	source := domain.SourceNewToken
	txSig := "TestTxSig"
	eventIndex := 0
	slot := int64(1000000)

	// Compute multiple times
	results := make([]string, 10)
	for i := 0; i < 10; i++ {
		results[i] = ComputeCandidateID(mint, pool, source, txSig, eventIndex, slot)
	}

	// All should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("Determinism failed: results[%d]=%s != results[0]=%s", i, results[i], results[0])
		}
	}
}

func TestComputeCandidateID_DifferentInputs(t *testing.T) {
	pool := strPtr("Pool")
	base := ComputeCandidateID("Mint", pool, domain.SourceNewToken, "Tx", 0, 1000)

	// Different mint should produce different hash
	diffMint := ComputeCandidateID("DifferentMint", pool, domain.SourceNewToken, "Tx", 0, 1000)
	if base == diffMint {
		t.Error("Different mint should produce different hash")
	}

	// Different source should produce different hash
	diffSource := ComputeCandidateID("Mint", pool, domain.SourceActiveToken, "Tx", 0, 1000)
	if base == diffSource {
		t.Error("Different source should produce different hash")
	}

	// Different slot should produce different hash
	diffSlot := ComputeCandidateID("Mint", pool, domain.SourceNewToken, "Tx", 0, 2000)
	if base == diffSlot {
		t.Error("Different slot should produce different hash")
	}

	// Different event_index should produce different hash
	diffEvent := ComputeCandidateID("Mint", pool, domain.SourceNewToken, "Tx", 1, 1000)
	if base == diffEvent {
		t.Error("Different event_index should produce different hash")
	}
}

func strPtr(s string) *string {
	return &s
}
