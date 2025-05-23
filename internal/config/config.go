package config

import (
	"fmt"
	"os"

	"github.com/zeek-r/go-conductor/internal/logger"
	"gopkg.in/yaml.v3"
)

// Config holds the main application configuration
type Config struct {
	Port     int           `yaml:"port"`
	Services []Service     `yaml:"services"`
	Timeout  int           `yaml:"timeout,omitempty"` // Timeout in seconds for requests
	Logging  logger.Config `yaml:"logging,omitempty"` // Logging configuration
	Metrics  MetricsConfig `yaml:"metrics,omitempty"` // Metrics configuration
}

// Service defines a backend service to proxy to
type Service struct {
	Name       string            `yaml:"name"`
	URL        string            `yaml:"url"`
	Path       string            `yaml:"path"`
	PathPrefix string            `yaml:"pathPrefix,omitempty"`
	PathExact  string            `yaml:"pathExact,omitempty"`
	Primary    bool              `yaml:"primary,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	Weight     int               `yaml:"weight,omitempty"` // For future use with load balancing
}

// MetricsConfig defines how metrics are collected and exposed
type MetricsConfig struct {
	Enabled          bool   `yaml:"enabled"`          // Whether metrics collection is enabled
	Endpoint         string `yaml:"endpoint"`         // Endpoint path to expose metrics (e.g., /metrics)
	EnablePrometheus bool   `yaml:"enablePrometheus"` // Enable Prometheus format metrics
}

// Load reads the configuration from the specified file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Set default port if not specified
	if config.Port == 0 {
		config.Port = 8080
	}

	// Set default timeout if not specified
	if config.Timeout == 0 {
		config.Timeout = 30 // 30 seconds
	}

	// Set default metrics settings if enabled but not configured
	if config.Metrics.Enabled {
		if config.Metrics.Endpoint == "" {
			config.Metrics.Endpoint = "/metrics"
		}
	}

	// Validate that at least one service is marked as primary
	primaryFound := false
	for _, service := range config.Services {
		if service.Primary {
			primaryFound = true
			break
		}
	}

	if !primaryFound && len(config.Services) > 0 {
		// If no service is explicitly marked as primary, set the first one
		config.Services[0].Primary = true
	}

	return &config, nil
}
