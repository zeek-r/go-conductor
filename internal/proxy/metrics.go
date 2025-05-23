package proxy

import (
	"sync"
	"time"
)

// MetricsCollector collects metrics about proxy operations
type MetricsCollector struct {
	mu              sync.RWMutex
	requestCount    int64
	errorCount      int64
	requestDuration time.Duration
	lastRequest     time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		lastRequest: time.Now(),
	}
}

// RecordRequest records metrics for a request
func (m *MetricsCollector) RecordRequest(duration time.Duration, isError bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCount++
	m.requestDuration += duration
	m.lastRequest = time.Now()

	if isError {
		m.errorCount++
	}
}

// GetRequestCount returns the total number of requests processed
func (m *MetricsCollector) GetRequestCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestCount
}

// GetErrorCount returns the total number of errors encountered
func (m *MetricsCollector) GetErrorCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errorCount
}

// GetAverageRequestDuration returns the average duration of requests
func (m *MetricsCollector) GetAverageRequestDuration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.requestCount == 0 {
		return 0
	}

	return m.requestDuration / time.Duration(m.requestCount)
}

// GetLastRequestTime returns the time of the last request
func (m *MetricsCollector) GetLastRequestTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastRequest
}
