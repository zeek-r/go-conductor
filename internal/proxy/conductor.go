package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zeek-r/go-conductor/internal/config"
	"github.com/zeek-r/go-conductor/internal/logger"
)

// Conductor handles incoming requests and routes them to the appropriate backend services
type Conductor struct {
	services       []*Service
	client         *http.Client
	timeout        time.Duration
	routesByPrefix map[string][]*Service
	routesByExact  map[string][]*Service
	routesByPath   map[string][]*Service
}

// Service represents a backend service with its configuration
type Service struct {
	Name    string
	URL     *url.URL
	Path    string
	Primary bool
	Config  config.Service
}

// serviceResult holds the result from a service request
type serviceResult struct {
	service *Service
	resp    *http.Response
	body    []byte
	err     error
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
	}

	// Initialize services
	conductor.initializeServices(cfg.Services)

	return conductor
}

// initializeServices sets up service routing based on configuration
func (c *Conductor) initializeServices(servicesConfig []config.Service) {
	for i, svcConfig := range servicesConfig {
		targetURL, err := url.Parse(svcConfig.URL)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Invalid target URL %s", svcConfig.URL), err)
		}

		service := &Service{
			Name:    svcConfig.Name,
			URL:     targetURL,
			Path:    svcConfig.Path,
			Primary: svcConfig.Primary,
			Config:  svcConfig,
		}

		c.services[i] = service

		// Register service by path type for easier lookup
		if svcConfig.PathExact != "" {
			c.routesByExact[svcConfig.PathExact] = append(
				c.routesByExact[svcConfig.PathExact], service)
		} else if svcConfig.PathPrefix != "" {
			c.routesByPrefix[svcConfig.PathPrefix] = append(
				c.routesByPrefix[svcConfig.PathPrefix], service)
		} else if svcConfig.Path != "" {
			c.routesByPath[svcConfig.Path] = append(
				c.routesByPath[svcConfig.Path], service)
		}
	}
}

// ServeHTTP implements the http.Handler interface
func (c *Conductor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	// Find matching services
	services := c.findMatchingServices(r)
	if len(services) == 0 {
		c.handleNoServiceFound(w, r)
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
		return
	}

	// Send the response back to the client
	c.writeResponse(w, resultToUse, r, requestStart)
}

// handleNoServiceFound handles the case when no service matches the request
func (c *Conductor) handleNoServiceFound(w http.ResponseWriter, r *http.Request) {
	logger.WarnWithFields("No service found for request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})
	http.Error(w, "No service found for request", http.StatusNotFound)
}

// readRequestBody reads the request body and returns it as a byte slice
func (c *Conductor) readRequestBody(r *http.Request) ([]byte, error) {
	var requestBody []byte
	if r.Body != nil {
		var err error
		requestBody, err = io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			return nil, err
		}
	}
	return requestBody, nil
}

// processResults processes the results from all services and returns the one to use
func (c *Conductor) processResults(resultChan <-chan *serviceResult, r *http.Request) *serviceResult {
	var primaryResult *serviceResult
	var anyResult *serviceResult

	for result := range resultChan {
		if result.err != nil {
			logger.ErrorWithFields("Error from service", result.err, map[string]interface{}{
				"service": result.service.Name,
				"method":  r.Method,
				"path":    r.URL.Path,
			})
			continue
		}

		// Keep track of any successful result as fallback
		if anyResult == nil {
			anyResult = result
		}

		// If this is from the primary service, we'll use this
		if result.service.Primary {
			primaryResult = result
			break
		}
	}

	// Use primary result if available, otherwise use any successful result
	if primaryResult != nil {
		logger.InfoWithFields("Using response from primary service", map[string]interface{}{
			"service":      primaryResult.service.Name,
			"status_code":  primaryResult.resp.StatusCode,
			"response_len": len(primaryResult.body),
			"method":       r.Method,
			"path":         r.URL.Path,
		})
		return primaryResult
	} else if anyResult != nil {
		logger.WarnWithFields("Primary service did not respond, using response from secondary service",
			map[string]interface{}{
				"service":      anyResult.service.Name,
				"status_code":  anyResult.resp.StatusCode,
				"response_len": len(anyResult.body),
				"method":       r.Method,
				"path":         r.URL.Path,
			})
		return anyResult
	}

	return nil
}

// writeResponse writes the service response back to the client
func (c *Conductor) writeResponse(w http.ResponseWriter, result *serviceResult, r *http.Request, requestStart time.Time) {
	// Copy response headers
	for k, values := range result.resp.Header {
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}

	// Set status code
	w.WriteHeader(result.resp.StatusCode)

	// Copy response body
	if result.body != nil {
		w.Write(result.body)
	}

	// Log request completion
	logger.DebugWithFields("Request completed", map[string]interface{}{
		"method":       r.Method,
		"path":         r.URL.Path,
		"status_code":  result.resp.StatusCode,
		"service_used": result.service.Name,
		"duration_ms":  time.Since(requestStart).Milliseconds(),
	})
}

// Helper function to get a list of service names
func getServiceNames(services []*Service) []string {
	names := make([]string, len(services))
	for i, svc := range services {
		names[i] = svc.Name
	}
	return names
}

// fanOutRequests sends the request to all services and returns a channel for the results
func (c *Conductor) fanOutRequests(ctx context.Context, services []*Service, originalReq *http.Request, requestBody []byte) <-chan *serviceResult {
	resultChan := make(chan *serviceResult, len(services))
	var wg sync.WaitGroup

	for _, service := range services {
		wg.Add(1)
		go func(svc *Service) {
			defer wg.Done()
			result := c.makeServiceRequest(ctx, svc, originalReq, requestBody)
			resultChan <- result
		}(service)
	}

	// Close the channel once all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan
}

// makeServiceRequest makes a request to a single service and returns the result
func (c *Conductor) makeServiceRequest(ctx context.Context, svc *Service, originalReq *http.Request, requestBody []byte) *serviceResult {
	// Create a new request for this service
	targetURL := c.createTargetURL(svc, originalReq)

	logger.DebugWithFields("Proxying request", map[string]interface{}{
		"service":     svc.Name,
		"target_url":  targetURL,
		"source_path": originalReq.URL.Path,
	})

	// Create request with provided body
	req, err := http.NewRequestWithContext(ctx, originalReq.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return &serviceResult{service: svc, err: err}
	}

	// Copy headers and add custom ones
	c.copyAndAugmentHeaders(req, originalReq, svc)

	// Send request and process response
	return c.sendRequest(svc, req, targetURL)
}

// copyAndAugmentHeaders copies the original request headers and adds service-specific headers
func (c *Conductor) copyAndAugmentHeaders(req *http.Request, originalReq *http.Request, svc *Service) {
	// Copy original headers
	for k, values := range originalReq.Header {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	// Add custom headers for this service
	for k, v := range svc.Config.Headers {
		req.Header.Set(k, v)
	}
}

// sendRequest sends the HTTP request and returns the result
func (c *Conductor) sendRequest(svc *Service, req *http.Request, targetURL string) *serviceResult {
	requestStart := time.Now()
	resp, err := c.client.Do(req)
	requestDuration := time.Since(requestStart)

	if err != nil {
		logger.ErrorWithFields("Request to service failed", err, map[string]interface{}{
			"service":     svc.Name,
			"target_url":  targetURL,
			"duration_ms": requestDuration.Milliseconds(),
		})
		return &serviceResult{service: svc, err: err}
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &serviceResult{service: svc, resp: resp, err: err}
	}

	logger.DebugWithFields("Service response received", map[string]interface{}{
		"service":      svc.Name,
		"status_code":  resp.StatusCode,
		"duration_ms":  requestDuration.Milliseconds(),
		"response_len": len(body),
	})

	return &serviceResult{
		service: svc,
		resp:    resp,
		body:    body,
		err:     nil,
	}
}

// createTargetURL creates the target URL for the proxy request
func (c *Conductor) createTargetURL(svc *Service, originalReq *http.Request) string {
	targetURL := svc.URL.String()

	// Determine path to use based on route type
	path := originalReq.URL.Path
	if svc.Config.PathPrefix != "" && strings.HasPrefix(path, svc.Config.PathPrefix) {
		// Strip the prefix from the path
		path = strings.TrimPrefix(path, svc.Config.PathPrefix)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	// Ensure the target URL has a trailing slash before appending path
	if !strings.HasSuffix(targetURL, "/") && path != "" && !strings.HasPrefix(path, "/") {
		targetURL += "/"
	}

	// Build the final target URL
	if strings.HasPrefix(path, "/") {
		targetURL = svc.URL.Scheme + "://" + svc.URL.Host + path
	} else if path != "" {
		targetURL = svc.URL.Scheme + "://" + svc.URL.Host + "/" + path
	}

	// Add query parameters if any
	if originalReq.URL.RawQuery != "" {
		targetURL += "?" + originalReq.URL.RawQuery
	}

	return targetURL
}

// findMatchingServices returns all services that match the request path
func (c *Conductor) findMatchingServices(r *http.Request) []*Service {
	path := r.URL.Path
	var matches []*Service

	// First, check for exact path matches
	if services, ok := c.routesByExact[path]; ok {
		return services
	}

	// Then, check for prefix matches (longest prefix wins)
	var bestPrefix string
	for prefix, services := range c.routesByPrefix {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(bestPrefix) {
			bestPrefix = prefix
			matches = services
		}
	}

	if len(matches) > 0 {
		return matches
	}

	// Finally, check for normal path matches
	for basePath, services := range c.routesByPath {
		if strings.HasPrefix(path, basePath) {
			matches = append(matches, services...)
		}
	}

	return matches
}
