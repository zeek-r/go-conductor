package proxy

import (
	"net/http"

	"github.com/zeek-r/go-conductor/internal/logger"
)

// SetupMetricsEndpoints configures and registers metrics endpoints based on the given configuration
func SetupMetricsEndpoints(mux *http.ServeMux, conductor *Conductor) {
	cfg := conductor.config
	if cfg == nil || !cfg.Metrics.Enabled {
		return
	}

	endpoint := cfg.Metrics.Endpoint
	if endpoint == "" {
		endpoint = "/metrics"
	}

	// If Prometheus is enabled, register its handler
	if cfg.Metrics.EnablePrometheus {
		logger.InfoWithFields("Enabling Prometheus metrics endpoint", map[string]interface{}{
			"endpoint": endpoint,
		})

		RegisterPrometheusEndpoint(mux, conductor, endpoint)
	} else {
		// Otherwise use the legacy JSON metrics handler
		logger.InfoWithFields("Enabling JSON metrics endpoint", map[string]interface{}{
			"endpoint": endpoint,
		})

		// Make sure metrics collector is enabled
		if conductor.GetMetrics() == nil {
			WithMetrics(conductor)
		}

		// Register the legacy metrics handler
		mux.HandleFunc(endpoint, MetricsHandler(conductor))
	}
}
