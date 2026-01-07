// Package observability provides Prometheus metrics for monitoring.
package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	// Ingestion metrics
	SwapEventsProcessed      prometheus.Counter
	LiquidityEventsProcessed prometheus.Counter
	SwapEventsStored         prometheus.Counter
	LiquidityEventsStored    prometheus.Counter
	EventProcessingErrors    *prometheus.CounterVec

	// Discovery metrics
	NewTokensDiscovered    prometheus.Counter
	ActiveTokensDiscovered prometheus.Counter
	CandidatesCreated      *prometheus.CounterVec

	// Buffer metrics
	SwapBufferSize      prometheus.Gauge
	LiquidityBufferSize prometheus.Gauge
	HighestSlotSeen     prometheus.Gauge

	// Latency metrics
	EventProcessingLatency *prometheus.HistogramVec
	RPCCallLatency         *prometheus.HistogramVec
	WSMessageLatency       prometheus.Histogram

	// Pipeline metrics
	PipelineRunsTotal    *prometheus.CounterVec
	PipelineDuration     *prometheus.HistogramVec
	TradesSimulated      prometheus.Counter
	AggregatesComputed   prometheus.Counter
	ReportsGenerated     prometheus.Counter

	// Database metrics
	DBQueryDuration *prometheus.HistogramVec
	DBQueryErrors   *prometheus.CounterVec
	DBConnections   *prometheus.GaugeVec

	// Health metrics
	LastSuccessfulIngestion prometheus.Gauge
	LastSuccessfulPipeline  prometheus.Gauge
	UptimeSeconds           prometheus.Counter
}

// NewMetrics creates a new Metrics instance with all metrics registered.
func NewMetrics(namespace string) *Metrics {
	if namespace == "" {
		namespace = "solana_token_lab"
	}

	return &Metrics{
		// Ingestion metrics
		SwapEventsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "swap_events_processed_total",
			Help:      "Total number of swap events processed",
		}),
		LiquidityEventsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "liquidity_events_processed_total",
			Help:      "Total number of liquidity events processed",
		}),
		SwapEventsStored: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "swap_events_stored_total",
			Help:      "Total number of swap events stored to database",
		}),
		LiquidityEventsStored: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "liquidity_events_stored_total",
			Help:      "Total number of liquidity events stored to database",
		}),
		EventProcessingErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "event_processing_errors_total",
			Help:      "Total number of event processing errors by type",
		}, []string{"event_type", "error_type"}),

		// Discovery metrics
		NewTokensDiscovered: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "discovery",
			Name:      "new_tokens_discovered_total",
			Help:      "Total number of NEW_TOKEN candidates discovered",
		}),
		ActiveTokensDiscovered: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "discovery",
			Name:      "active_tokens_discovered_total",
			Help:      "Total number of ACTIVE_TOKEN candidates discovered",
		}),
		CandidatesCreated: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "discovery",
			Name:      "candidates_created_total",
			Help:      "Total number of candidates created by source",
		}, []string{"source"}),

		// Buffer metrics
		SwapBufferSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "swap_buffer_size",
			Help:      "Current number of slots in swap event buffer",
		}),
		LiquidityBufferSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "liquidity_buffer_size",
			Help:      "Current number of slots in liquidity event buffer",
		}),
		HighestSlotSeen: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "highest_slot_seen",
			Help:      "Highest Solana slot number seen",
		}),

		// Latency metrics
		EventProcessingLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "ingestion",
			Name:      "event_processing_latency_seconds",
			Help:      "Event processing latency in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"event_type"}),
		RPCCallLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "solana",
			Name:      "rpc_call_latency_seconds",
			Help:      "Solana RPC call latency in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),
		WSMessageLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "solana",
			Name:      "ws_message_latency_seconds",
			Help:      "WebSocket message processing latency in seconds",
			Buckets:   prometheus.DefBuckets,
		}),

		// Pipeline metrics
		PipelineRunsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "pipeline",
			Name:      "runs_total",
			Help:      "Total number of pipeline runs by status",
		}, []string{"phase", "status"}),
		PipelineDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "pipeline",
			Name:      "duration_seconds",
			Help:      "Pipeline execution duration in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		}, []string{"phase"}),
		TradesSimulated: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "pipeline",
			Name:      "trades_simulated_total",
			Help:      "Total number of trades simulated",
		}),
		AggregatesComputed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "pipeline",
			Name:      "aggregates_computed_total",
			Help:      "Total number of strategy aggregates computed",
		}),
		ReportsGenerated: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "pipeline",
			Name:      "reports_generated_total",
			Help:      "Total number of reports generated",
		}),

		// Database metrics
		DBQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"database", "operation"}),
		DBQueryErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "query_errors_total",
			Help:      "Total number of database query errors",
		}, []string{"database", "operation"}),
		DBConnections: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "database",
			Name:      "connections",
			Help:      "Number of database connections by state",
		}, []string{"database", "state"}),

		// Health metrics
		LastSuccessfulIngestion: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "health",
			Name:      "last_successful_ingestion_timestamp",
			Help:      "Unix timestamp of last successful ingestion",
		}),
		LastSuccessfulPipeline: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "health",
			Name:      "last_successful_pipeline_timestamp",
			Help:      "Unix timestamp of last successful pipeline run",
		}),
		UptimeSeconds: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "health",
			Name:      "uptime_seconds_total",
			Help:      "Total uptime in seconds",
		}),
	}
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// DefaultMetrics is the default metrics instance.
var DefaultMetrics = NewMetrics("")

// RecordSwapProcessed increments the swap events processed counter.
func RecordSwapProcessed() {
	DefaultMetrics.SwapEventsProcessed.Inc()
}

// RecordLiquidityProcessed increments the liquidity events processed counter.
func RecordLiquidityProcessed() {
	DefaultMetrics.LiquidityEventsProcessed.Inc()
}

// RecordNewTokenDiscovered increments the new tokens discovered counter.
func RecordNewTokenDiscovered() {
	DefaultMetrics.NewTokensDiscovered.Inc()
	DefaultMetrics.CandidatesCreated.WithLabelValues("NEW_TOKEN").Inc()
}

// RecordActiveTokenDiscovered increments the active tokens discovered counter.
func RecordActiveTokenDiscovered() {
	DefaultMetrics.ActiveTokensDiscovered.Inc()
	DefaultMetrics.CandidatesCreated.WithLabelValues("ACTIVE_TOKEN").Inc()
}

// RecordEventError records an event processing error.
func RecordEventError(eventType, errorType string) {
	DefaultMetrics.EventProcessingErrors.WithLabelValues(eventType, errorType).Inc()
}

// UpdateBufferSizes updates the buffer size gauges.
func UpdateBufferSizes(swapSlots, liquiditySlots int) {
	DefaultMetrics.SwapBufferSize.Set(float64(swapSlots))
	DefaultMetrics.LiquidityBufferSize.Set(float64(liquiditySlots))
}

// UpdateHighestSlot updates the highest slot seen gauge.
func UpdateHighestSlot(slot int64) {
	DefaultMetrics.HighestSlotSeen.Set(float64(slot))
}

// RecordRPCLatency records RPC call latency.
func RecordRPCLatency(method string, seconds float64) {
	DefaultMetrics.RPCCallLatency.WithLabelValues(method).Observe(seconds)
}

// RecordDBQuery records database query metrics.
func RecordDBQuery(database, operation string, seconds float64, err error) {
	DefaultMetrics.DBQueryDuration.WithLabelValues(database, operation).Observe(seconds)
	if err != nil {
		DefaultMetrics.DBQueryErrors.WithLabelValues(database, operation).Inc()
	}
}

// RecordPipelineRun records a pipeline run.
func RecordPipelineRun(phase, status string, durationSeconds float64) {
	DefaultMetrics.PipelineRunsTotal.WithLabelValues(phase, status).Inc()
	DefaultMetrics.PipelineDuration.WithLabelValues(phase).Observe(durationSeconds)
}
