package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusCollector implements Collector using Prometheus.
type PrometheusCollector struct {
	sseConnectionsCurrent prometheus.Gauge
	sseConnectionsTotal   prometheus.Counter
	toolCallsTotal        *prometheus.CounterVec
}

var (
	prometheusOnce     sync.Once
	prometheusInstance *PrometheusCollector
)

// NewPrometheusCollector creates a new Prometheus-based collector.
// The collector is a singleton to prevent duplicate metric registration panics.
func NewPrometheusCollector() Collector {
	prometheusOnce.Do(func() {
		sseConnectionsCurrent := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "searchmcp_sse_connections_current",
			Help: "Current number of active SSE connections",
		})

		sseConnectionsTotal := prometheus.NewCounter(prometheus.CounterOpts{
			Name: "searchmcp_sse_connections_total",
			Help: "Total number of SSE connections since server start",
		})

		toolCallsTotal := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "searchmcp_tool_calls_total",
				Help: "Total number of tool calls by tool name",
			},
			[]string{"tool"},
		)

		prometheus.MustRegister(sseConnectionsCurrent)
		prometheus.MustRegister(sseConnectionsTotal)
		prometheus.MustRegister(toolCallsTotal)

		prometheusInstance = &PrometheusCollector{
			sseConnectionsCurrent: sseConnectionsCurrent,
			sseConnectionsTotal:   sseConnectionsTotal,
			toolCallsTotal:        toolCallsTotal,
		}
	})

	return prometheusInstance
}

func (c *PrometheusCollector) RecordSSEConnection() {
	c.sseConnectionsCurrent.Inc()
	c.sseConnectionsTotal.Inc()
}

func (c *PrometheusCollector) RecordSSEDisconnection() {
	c.sseConnectionsCurrent.Dec()
}

func (c *PrometheusCollector) RecordToolCall(toolName string) {
	c.toolCallsTotal.WithLabelValues(toolName).Inc()
}

func (c *PrometheusCollector) Handler() http.Handler {
	return promhttp.Handler()
}
