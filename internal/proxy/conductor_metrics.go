package proxy

import (
	"time"
)

// WithMetrics adds metrics collection capability to a conductor
func WithMetrics(c *Conductor) *Conductor {
	c.metrics = NewMetricsCollector()
	return c
}

// RecordMetrics records metrics for a request
func (c *Conductor) RecordMetrics(start time.Time, hasError bool) {
	if c.metrics == nil {
		return
	}
	duration := time.Since(start)
	c.metrics.RecordRequest(duration, hasError)
}

// GetMetrics returns the metrics collector for this conductor
// Returns nil if metrics collection is not enabled
func (c *Conductor) GetMetrics() *MetricsCollector {
	return c.metrics
}
