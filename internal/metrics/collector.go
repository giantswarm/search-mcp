package metrics

import "net/http"

// Collector defines the interface for collecting metrics.
// All methods are thread-safe and should never panic.
type Collector interface {
	// RecordSSEConnection records a new SSE connection.
	// Increments both current connections gauge and total connections counter.
	RecordSSEConnection()

	// RecordSSEDisconnection records an SSE disconnection.
	// Decrements current connections gauge.
	RecordSSEDisconnection()

	// RecordToolCall records a tool invocation.
	// Increments the tool calls counter for the specified tool name.
	RecordToolCall(toolName string)

	// Handler returns an HTTP handler for the /metrics endpoint.
	// Returns nil for no-op implementations.
	Handler() http.Handler
}
