package proxy

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/zeek-r/go-conductor/internal/config"
	"github.com/zeek-r/go-conductor/internal/logger"
)

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

// Helper function to get a list of service names
func getServiceNames(services []*Service) []string {
	names := make([]string, len(services))
	for i, svc := range services {
		names[i] = svc.Name
	}
	return names
} 