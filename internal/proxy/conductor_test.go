package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zeek-r/go-conductor/internal/config"
)

// mockTransport is used to mock HTTP responses for testing
type mockTransport struct {
	responseMap map[string]*http.Response
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.responseMap[req.URL.String()], nil
}

// createTestConductor creates a Conductor with mock services for testing
func createTestConductor() *Conductor {
	cfg := &config.Config{
		Port:    8080,
		Timeout: 5,
		Services: []config.Service{
			{
				Name:      "primary-service",
				URL:       "http://primary.example.com",
				PathExact: "/exact",
				Primary:   true,
			},
			{
				Name:       "prefix-service",
				URL:        "http://prefix.example.com",
				PathPrefix: "/prefix",
			},
			{
				Name: "path-service",
				URL:  "http://path.example.com",
				Path: "/path",
			},
		},
	}

	conductor := NewConductor(cfg)

	// Replace the HTTP client with a mock
	transport := &mockTransport{
		responseMap: map[string]*http.Response{
			"http://primary.example.com/exact": {
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"message":"primary response"}`)),
			},
			"http://prefix.example.com/api": {
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"message":"prefix response"}`)),
			},
			"http://path.example.com/path/to/resource": {
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"message":"path response"}`)),
			},
		},
	}

	conductor.client = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}

	return conductor
}

// TestFindMatchingServices tests the findMatchingServices function
func TestFindMatchingServices(t *testing.T) {
	conductor := createTestConductor()

	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{
			name:     "exact path",
			path:     "/exact",
			expected: 1,
		},
		{
			name:     "prefix path",
			path:     "/prefix/api",
			expected: 1,
		},
		{
			name:     "path match",
			path:     "/path/to/resource",
			expected: 1,
		},
		{
			name:     "no match",
			path:     "/unknown",
			expected: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com"+test.path, nil)
			services := conductor.findMatchingServices(req)

			if len(services) != test.expected {
				t.Errorf("Expected %d services, got %d", test.expected, len(services))
			}
		})
	}
}

// TestCreateTargetURL tests the createTargetURL function
func TestCreateTargetURL(t *testing.T) {
	conductor := createTestConductor()

	tests := []struct {
		name            string
		path            string
		serviceIndex    int
		expectedContain string
	}{
		{
			name:            "exact path",
			path:            "/exact",
			serviceIndex:    0,
			expectedContain: "http://primary.example.com/exact",
		},
		{
			name:            "prefix path",
			path:            "/prefix/api",
			serviceIndex:    1,
			expectedContain: "http://prefix.example.com/api",
		},
		{
			name:            "path match",
			path:            "/path/to/resource",
			serviceIndex:    2,
			expectedContain: "http://path.example.com/path/to/resource",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com"+test.path, nil)
			service := conductor.services[test.serviceIndex]
			targetURL := conductor.createTargetURL(service, req)

			if !strings.Contains(targetURL, test.expectedContain) {
				t.Errorf("Expected URL to contain %s, got %s", test.expectedContain, targetURL)
			}
		})
	}
}

// TestMakeServiceRequest tests the makeServiceRequest function
func TestMakeServiceRequest(t *testing.T) {
	conductor := createTestConductor()

	tests := []struct {
		name         string
		path         string
		serviceIndex int
		expectError  bool
		expectStatus int
	}{
		{
			name:         "primary service request",
			path:         "/exact",
			serviceIndex: 0,
			expectError:  false,
			expectStatus: 200,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com"+test.path, nil)
			ctx := context.Background()
			service := conductor.services[test.serviceIndex]

			result := conductor.makeServiceRequest(ctx, service, req, nil)

			if test.expectError && result.err == nil {
				t.Errorf("Expected error, but got nil")
			}

			if !test.expectError && result.err != nil {
				t.Errorf("Expected no error, but got: %v", result.err)
			}

			if !test.expectError && result.resp.StatusCode != test.expectStatus {
				t.Errorf("Expected status %d, got %d", test.expectStatus, result.resp.StatusCode)
			}
		})
	}
}

// TestServeHTTP tests the ServeHTTP function end-to-end
func TestServeHTTP(t *testing.T) {
	conductor := createTestConductor()

	tests := []struct {
		name         string
		path         string
		expectStatus int
		expectBody   string
	}{
		{
			name:         "exact path match",
			path:         "/exact",
			expectStatus: 200,
			expectBody:   `{"message":"primary response"}`,
		},
		{
			name:         "no match",
			path:         "/unknown",
			expectStatus: 404,
			expectBody:   "No service found for request\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com"+test.path, nil)
			recorder := httptest.NewRecorder()

			// Call ServeHTTP
			conductor.ServeHTTP(recorder, req)

			// Check status code
			if recorder.Code != test.expectStatus {
				t.Errorf("Expected status %d, got %d", test.expectStatus, recorder.Code)
			}

			// Check response body
			body := recorder.Body.String()
			if !strings.Contains(body, test.expectBody) {
				t.Errorf("Expected body to contain: %s, got: %s", test.expectBody, body)
			}
		})
	}
}
