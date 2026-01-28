package metrics

import "net/http"

// NoopCollector is a no-op implementation of Collector.
// Used when metrics are disabled (e.g., stdio mode).
type NoopCollector struct{}

// NewNoopCollector creates a new no-op collector.
func NewNoopCollector() Collector {
	return &NoopCollector{}
}

func (n *NoopCollector) RecordSSEConnection()           {}
func (n *NoopCollector) RecordSSEDisconnection()        {}
func (n *NoopCollector) RecordToolCall(toolName string) {}
func (n *NoopCollector) Handler() http.Handler          { return nil }
