package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "omegle_connections_active",
		Help: "Number of currently open WebSocket connections.",
	})
	QueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "omegle_queue_depth",
		Help: "Number of sessions currently waiting to be matched.",
	})
	MatchesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "omegle_matches_total",
		Help: "Total number of matches made.",
	}, []string{"kind"}) // kind = tagged|fallback
	MatchWaitMs = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "omegle_match_wait_ms",
		Help:    "Time spent waiting for a match, in milliseconds.",
		Buckets: prometheus.ExponentialBuckets(50, 2, 12),
	})
	MessagesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "omegle_messages_total",
		Help: "Total chat messages relayed.",
	})
	ErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "omegle_errors_total",
		Help: "Total protocol errors emitted by code.",
	}, []string{"code"})
)

func init() {
	prometheus.MustRegister(ConnectionsActive, QueueDepth, MatchesTotal, MatchWaitMs, MessagesTotal, ErrorsTotal)
}

// Handler returns the HTTP handler that exposes /metrics in Prometheus text format.
func Handler() http.Handler { return promhttp.Handler() }
