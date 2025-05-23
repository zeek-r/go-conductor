package proxy

import (
	"net/http"
	"strings"
)

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