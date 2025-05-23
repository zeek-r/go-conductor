package proxy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/zeek-r/go-conductor/internal/config"
	"github.com/zeek-r/go-conductor/internal/logger"
)

// Conductor handles incoming requests and routes them to the appropriate backend services
type Conductor struct {
	services          []*Service
	client            *http.Client
	timeout           time.Duration
	routesByPrefix    map[string][]*Service
	routesByExact     map[string][]*Service
	routesByPath      map[string][]*Service
	metrics           *MetricsCollector  // Legacy metrics collector
	prometheusMetrics *PrometheusMetrics // Prometheus metrics collector
	config            *config.Config     // Reference to configuration
}

// NewConductor creates a new Conductor with the provided configuration
func NewConductor(cfg *config.Config) *Conductor {
	timeout := time.Duration(cfg.Timeout) * time.Second
	client := &http.Client{
		Timeout: timeout,
	}

	conductor := &Conductor{
		services:       make([]*Service, len(cfg.Services)),
		client:         client,
		timeout:        timeout,
		routesByPrefix: make(map[string][]*Service),
		routesByExact:  make(map[string][]*Service),
		routesByPath:   make(map[string][]*Service),
		config:         cfg,
	}

	// Initialize services
	conductor.initializeServices(cfg.Services)

	// Setup metrics if enabled
	if cfg.Metrics.Enabled {
		// Legacy metrics collector is always initialized when metrics are enabled
		WithMetrics(conductor)

		// Initialize Prometheus metrics if configured
		if cfg.Metrics.EnablePrometheus {
			WithPrometheusMetrics(conductor)
		}
	}

	return conductor
}

// ServeHTTP implements the http.Handler interface
func (c *Conductor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	// Track in-flight requests for Prometheus if enabled
	if c.prometheusMetrics != nil {
		c.prometheusMetrics.RequestStarted()
		defer c.prometheusMetrics.RequestFinished()
	}

	// Find matching services
	services := c.findMatchingServices(r)
	if len(services) == 0 {
		c.handleNoServiceFound(w, r)

		// Record not found error in Prometheus metrics
		if c.prometheusMetrics != nil {
			c.prometheusMetrics.RecordError("none", "no_service_found")
			c.prometheusMetrics.RecordRequest("none", r.Method, "404", time.Since(requestStart))
		}

		// Record metrics for legacy collector
		if c.metrics != nil {
			c.RecordMetrics(requestStart, true)
		}
		return
	}

	logger.InfoWithFields(fmt.Sprintf("Found %d matching service(s)", len(services)), map[string]interface{}{
		"method":        r.Method,
		"path":          r.URL.Path,
		"service_count": len(services),
		"services":      getServiceNames(services),
	})

	// Create a context with the configured timeout
	ctx, cancel := context.WithTimeout(r.Context(), c.timeout)
	defer cancel()

	// Read the body once so we can send it to multiple services
	requestBody, err := c.readRequestBody(r)
	if err != nil {
		logger.ErrorWithFields("Failed to read request body", err, map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		})
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)

		// Record error in Prometheus metrics
		if c.prometheusMetrics != nil {
			c.prometheusMetrics.RecordError("conductor", "read_body_failed")
			c.prometheusMetrics.RecordRequest("conductor", r.Method, "500", time.Since(requestStart))
		}

		// Record metrics for legacy collector
		if c.metrics != nil {
			c.RecordMetrics(requestStart, true)
		}
		return
	}

	// Fan out requests to all matching services
	resultChan := c.fanOutRequests(ctx, services, r, requestBody)

	// Process results and select the appropriate response
	resultToUse := c.processResults(resultChan, r)
	if resultToUse == nil {
		logger.ErrorWithFields("All services failed", nil, map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		})
		http.Error(w, "All services failed", http.StatusBadGateway)

		// Record error in Prometheus metrics
		if c.prometheusMetrics != nil {
			c.prometheusMetrics.RecordError("all", "all_services_failed")
			c.prometheusMetrics.RecordRequest("all", r.Method, "502", time.Since(requestStart))
		}

		// Record metrics for legacy collector
		if c.metrics != nil {
			c.RecordMetrics(requestStart, true)
		}
		return
	}

	// Send the response back to the client
	c.writeResponse(w, resultToUse, r, requestStart)

	// Record successful request in Prometheus metrics
	if c.prometheusMetrics != nil {
		status := fmt.Sprintf("%d", resultToUse.resp.StatusCode)
		c.prometheusMetrics.RecordRequest(
			resultToUse.service.Name,
			r.Method,
			status,
			time.Since(requestStart),
		)
	}

	// Record metrics for legacy collector
	if c.metrics != nil {
		c.RecordMetrics(requestStart, false)
	}
}
