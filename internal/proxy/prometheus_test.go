package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zeek-r/go-conductor/internal/config"
)

func TestPrometheusMetrics(t *testing.T) {
	// Create a custom registry for this test
	registry := prometheus.NewRegistry()
	metrics := NewPrometheusMetrics(registry)

	// Test recording a request
	metrics.RecordRequest("test-service", "GET", "200", 100*time.Millisecond)
	metrics.RecordRequest("test-service", "POST", "201", 200*time.Millisecond)

	// Test recording an error
	metrics.RecordError("test-service", "timeout")

	// Test in-flight requests
	metrics.RequestStarted()
	metrics.RequestStarted()
	metrics.RequestFinished()

	// Test service health
	metrics.SetServiceHealth("test-service", true)
	metrics.SetServiceHealth("down-service", false)
}

func TestPrometheusEndpoint(t *testing.T) {
	// Create a test config with Prometheus metrics enabled
	cfg := &config.Config{
		Port:    8080,
		Timeout: 5,
		Services: []config.Service{
			{
				Name:      "test-service",
				URL:       "http://example.com",
				PathExact: "/test",
				Primary:   true,
			},
		},
		Metrics: config.MetricsConfig{
			Enabled:          true,
			EnablePrometheus: true,
			Endpoint:         "/metrics",
		},
	}

	// Create a custom registry for this test
	registry := prometheus.NewRegistry()

	// Create a conductor with custom Prometheus metrics
	conductor := NewConductor(cfg)

	// Replace the default metrics with our test metrics
	conductor.prometheusMetrics = NewPrometheusMetrics(registry)

	// Create a test HTTP server with a handler that uses the custom registry
	mux := http.NewServeMux()

	// Register a custom handler that uses our test registry
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// Create a test request to the metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	recorder := httptest.NewRecorder()

	// Record some test metrics
	if conductor.prometheusMetrics != nil {
		conductor.prometheusMetrics.RecordRequest("test-service", "GET", "200", 100*time.Millisecond)
		conductor.prometheusMetrics.RecordError("test-service", "test_error")
	}

	// Serve the request
	mux.ServeHTTP(recorder, req)

	// Check the response
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Verify the response contains Prometheus metrics format
	body := recorder.Body.String()

	// Check for expected metrics in the output
	expectedMetrics := []string{
		"go_conductor_requests_total",
		"go_conductor_errors_total",
		"go_conductor_request_duration_seconds",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric %s not found in response", metric)
		}
	}
}
