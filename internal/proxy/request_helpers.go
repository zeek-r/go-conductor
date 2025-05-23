package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/zeek-r/go-conductor/internal/logger"
)

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