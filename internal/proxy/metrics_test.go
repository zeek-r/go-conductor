package proxy

import (
	"testing"
	"time"
)

func TestMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()

	// Initial state
	if count := collector.GetRequestCount(); count != 0 {
		t.Errorf("Initial request count should be 0, got %d", count)
	}

	if errCount := collector.GetErrorCount(); errCount != 0 {
		t.Errorf("Initial error count should be 0, got %d", errCount)
	}

	if avgDuration := collector.GetAverageRequestDuration(); avgDuration != 0 {
		t.Errorf("Initial average duration should be 0, got %v", avgDuration)
	}

	// Record successful request
	collector.RecordRequest(100*time.Millisecond, false)

	if count := collector.GetRequestCount(); count != 1 {
		t.Errorf("Request count should be 1, got %d", count)
	}

	if errCount := collector.GetErrorCount(); errCount != 0 {
		t.Errorf("Error count should still be 0, got %d", errCount)
	}

	if avgDuration := collector.GetAverageRequestDuration(); avgDuration != 100*time.Millisecond {
		t.Errorf("Average duration should be 100ms, got %v", avgDuration)
	}

	// Record error request
	collector.RecordRequest(200*time.Millisecond, true)

	if count := collector.GetRequestCount(); count != 2 {
		t.Errorf("Request count should be 2, got %d", count)
	}

	if errCount := collector.GetErrorCount(); errCount != 1 {
		t.Errorf("Error count should be 1, got %d", errCount)
	}

	expectedAvg := 150 * time.Millisecond // (100ms + 200ms) / 2
	if avgDuration := collector.GetAverageRequestDuration(); avgDuration != expectedAvg {
		t.Errorf("Average duration should be %v, got %v", expectedAvg, avgDuration)
	}
}

func TestConductorWithMetrics(t *testing.T) {
	conductor := createTestConductor() // Reuse helper from conductor_test.go

	// Initially metrics should be nil
	if metrics := conductor.GetMetrics(); metrics != nil {
		t.Errorf("Metrics should be nil initially")
	}

	// Add metrics
	WithMetrics(conductor)

	// Now metrics should be available
	if metrics := conductor.GetMetrics(); metrics == nil {
		t.Errorf("Metrics should not be nil after WithMetrics")
	} else {
		// Check initial metrics state
		if count := metrics.GetRequestCount(); count != 0 {
			t.Errorf("Initial request count should be 0, got %d", count)
		}
	}

	// Record a request using helper method
	start := time.Now().Add(-100 * time.Millisecond) // Simulate request started 100ms ago
	conductor.RecordMetrics(start, false)

	// Verify metrics were recorded
	metrics := conductor.GetMetrics()
	if count := metrics.GetRequestCount(); count != 1 {
		t.Errorf("Request count should be 1, got %d", count)
	}

	if errCount := metrics.GetErrorCount(); errCount != 0 {
		t.Errorf("Error count should be 0, got %d", errCount)
	}

	// Test with error
	start = time.Now().Add(-200 * time.Millisecond)
	conductor.RecordMetrics(start, true)

	if count := metrics.GetRequestCount(); count != 2 {
		t.Errorf("Request count should be 2, got %d", count)
	}

	if errCount := metrics.GetErrorCount(); errCount != 1 {
		t.Errorf("Error count should be 1, got %d", errCount)
	}
}
