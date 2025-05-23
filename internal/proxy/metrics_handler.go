package proxy

import (
	"encoding/json"
	"net/http"
	"time"
)

// MetricsData represents the metrics data structure for JSON output
type MetricsData struct {
	RequestCount         int64     `json:"request_count"`
	ErrorCount           int64     `json:"error_count"`
	SuccessCount         int64     `json:"success_count"`
	ErrorRate            float64   `json:"error_rate"`
	AverageRequestTimeMs float64   `json:"avg_request_time_ms"`
	LastRequestTimestamp time.Time `json:"last_request_time"`
	UptimeSeconds        float64   `json:"uptime_seconds"`
	StartTime            time.Time `json:"start_time"`
}

// MetricsHandler creates an HTTP handler for exposing conductor metrics
func MetricsHandler(c *Conductor) http.HandlerFunc {
	startTime := time.Now()

	return func(w http.ResponseWriter, r *http.Request) {
		metrics := c.GetMetrics()
		if metrics == nil {
			http.Error(w, "Metrics collection not enabled", http.StatusNotFound)
			return
		}

		// Get metrics data
		reqCount := metrics.GetRequestCount()
		errCount := metrics.GetErrorCount()
		successCount := reqCount - errCount

		// Calculate error rate
		var errorRate float64
		if reqCount > 0 {
			errorRate = float64(errCount) / float64(reqCount)
		}

		// Get average duration in milliseconds
		avgDuration := metrics.GetAverageRequestDuration()
		avgDurationMs := float64(avgDuration) / float64(time.Millisecond)

		// Last request time
		lastReq := metrics.GetLastRequestTime()

		// Calculate uptime
		uptime := time.Since(startTime).Seconds()

		// Create response
		data := MetricsData{
			RequestCount:         reqCount,
			ErrorCount:           errCount,
			SuccessCount:         successCount,
			ErrorRate:            errorRate,
			AverageRequestTimeMs: avgDurationMs,
			LastRequestTimestamp: lastReq,
			UptimeSeconds:        uptime,
			StartTime:            startTime,
		}

		// Set content type
		w.Header().Set("Content-Type", "application/json")

		// Return JSON response
		json.NewEncoder(w).Encode(data)
	}
}

// RegisterMetricsEndpoint adds the /metrics endpoint to the given ServeMux
func RegisterMetricsEndpoint(mux *http.ServeMux, c *Conductor) {
	// Enable metrics if not already enabled
	if c.GetMetrics() == nil {
		WithMetrics(c)
	}

	// Register the metrics endpoint
	mux.HandleFunc("/metrics", MetricsHandler(c))
}
