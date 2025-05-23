package proxy

import (
	"net/http"
	"time"

	"github.com/zeek-r/go-conductor/internal/logger"
)

// handleNoServiceFound handles the case when no service matches the request
func (c *Conductor) handleNoServiceFound(w http.ResponseWriter, r *http.Request) {
	logger.WarnWithFields("No service found for request", map[string]interface{}{
		"method": r.Method,
		"path":   r.URL.Path,
	})
	http.Error(w, "No service found for request", http.StatusNotFound)
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