package domain

// ScenarioConfig represents execution scenario parameters.
// From EXECUTION_SCENARIOS.md.
type ScenarioConfig struct {
	ScenarioID     string  // "optimistic" | "realistic" | "pessimistic" | "degraded"
	DelayMs        int64   // execution delay in milliseconds
	SlippagePct    float64 // slippage percentage
	FeeSOL         float64 // base transaction fee in SOL
	PriorityFeeSOL float64 // priority fee in SOL
	MEVPenaltyPct  float64 // MEV penalty percentage
}

// Scenario ID constants
const (
	ScenarioOptimistic  = "optimistic"
	ScenarioRealistic   = "realistic"
	ScenarioPessimistic = "pessimistic"
	ScenarioDegraded    = "degraded"
)

// Predefined scenario configurations from EXECUTION_SCENARIOS.md
var (
	ScenarioConfigOptimistic = ScenarioConfig{
		ScenarioID:     ScenarioOptimistic,
		DelayMs:        100,
		SlippagePct:    0.5,
		FeeSOL:         0.000005,
		PriorityFeeSOL: 0,
		MEVPenaltyPct:  0,
	}

	ScenarioConfigRealistic = ScenarioConfig{
		ScenarioID:     ScenarioRealistic,
		DelayMs:        500,
		SlippagePct:    2.0,
		FeeSOL:         0.00001,
		PriorityFeeSOL: 0.0001,
		MEVPenaltyPct:  1.0,
	}

	ScenarioConfigPessimistic = ScenarioConfig{
		ScenarioID:     ScenarioPessimistic,
		DelayMs:        2000,
		SlippagePct:    5.0,
		FeeSOL:         0.0001,
		PriorityFeeSOL: 0.001,
		MEVPenaltyPct:  3.0,
	}

	ScenarioConfigDegraded = ScenarioConfig{
		ScenarioID:     ScenarioDegraded,
		DelayMs:        5000,
		SlippagePct:    10.0,
		FeeSOL:         0.001,
		PriorityFeeSOL: 0.01,
		MEVPenaltyPct:  5.0,
	}
)
