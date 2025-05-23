package proxy

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Default namespace for all metrics
	namespace = "go_conductor"
)

// PrometheusMetrics holds all the Prometheus metrics for the conductor
type PrometheusMetrics struct {
	requestsTotal      *prometheus.CounterVec
	requestDuration    *prometheus.HistogramVec
	errorsTotal        *prometheus.CounterVec
	inFlightRequests   prometheus.Gauge
	serviceHealthGauge *prometheus.GaugeVec
}

// NewPrometheusMetrics creates a new set of Prometheus metrics
func NewPrometheusMetrics(registry ...prometheus.Registerer) *PrometheusMetrics {
	// Use default registerer if none is provided
	var reg prometheus.Registerer = prometheus.DefaultRegisterer
	if len(registry) > 0 && registry[0] != nil {
		reg = registry[0]
	}

	// Create a factory with the provided registry
	factory := promauto.With(reg)

	return &PrometheusMetrics{
		requestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "requests_total",
				Help:      "Total number of requests processed by the conductor",
			},
			[]string{"service", "method", "status"},
		),
		requestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "request_duration_seconds",
				Help:      "Duration of requests to backend services in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"service", "method"},
		),
		errorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total number of errors encountered during requests",
			},
			[]string{"service", "error_type"},
		),
		inFlightRequests: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "in_flight_requests",
				Help:      "Number of requests currently being processed",
			},
		),
		serviceHealthGauge: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "service_health",
				Help:      "Health status of backend services (1=healthy, 0=unhealthy)",
			},
			[]string{"service"},
		),
	}
}

// RecordRequest records metrics for a completed request
func (p *PrometheusMetrics) RecordRequest(serviceName string, method string, status string, duration time.Duration) {
	p.requestsTotal.WithLabelValues(serviceName, method, status).Inc()
	p.requestDuration.WithLabelValues(serviceName, method).Observe(duration.Seconds())
}

// RecordError records an error encountered during a request
func (p *PrometheusMetrics) RecordError(serviceName string, errorType string) {
	p.errorsTotal.WithLabelValues(serviceName, errorType).Inc()
}

// RequestStarted increments the gauge for in-flight requests
func (p *PrometheusMetrics) RequestStarted() {
	p.inFlightRequests.Inc()
}

// RequestFinished decrements the gauge for in-flight requests
func (p *PrometheusMetrics) RequestFinished() {
	p.inFlightRequests.Dec()
}

// SetServiceHealth sets the health status for a service
func (p *PrometheusMetrics) SetServiceHealth(serviceName string, healthy bool) {
	var value float64
	if healthy {
		value = 1.0
	}
	p.serviceHealthGauge.WithLabelValues(serviceName).Set(value)
}

// WithPrometheusMetrics adds Prometheus metrics collection capability to a conductor
func WithPrometheusMetrics(c *Conductor, registry ...prometheus.Registerer) *Conductor {
	c.prometheusMetrics = NewPrometheusMetrics(registry...)
	return c
}

// RegisterPrometheusEndpoint adds the /metrics endpoint to the given ServeMux using Prometheus
func RegisterPrometheusEndpoint(mux *http.ServeMux, c *Conductor, endpoint string) {
	// If endpoint path is not specified, use default
	if endpoint == "" {
		endpoint = "/metrics"
	}

	// Register the Prometheus handler
	mux.Handle(endpoint, promhttp.Handler())
}
