package app

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zeek-r/go-conductor/internal/config"
	"github.com/zeek-r/go-conductor/internal/logger"
	"github.com/zeek-r/go-conductor/internal/proxy"
)

// Run starts the go-conductor application with the given command line arguments
func Run() {
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging (overrides config file setting)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with configuration
	// If verbose flag is set, override the log level
	if *verboseFlag && cfg.Logging.Level != logger.LevelDebug {
		cfg.Logging.Level = logger.LevelDebug
	}

	// If logging is not configured, use defaults with info level
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = logger.LevelInfo
	}

	logger.Initialize(cfg.Logging)

	// Create proxy conductor
	conductor := proxy.NewConductor(cfg)

	// Setup main server mux
	mainMux := http.NewServeMux()

	// Setup proxy as the main handler for all non-special paths
	mainMux.Handle("/", conductor)

	// Setup metrics endpoints if enabled
	if cfg.Metrics.Enabled {
		proxy.SetupMetricsEndpoints(mainMux, conductor)
		logger.InfoWithFields("Metrics collection enabled", map[string]interface{}{
			"endpoint":   cfg.Metrics.Endpoint,
			"prometheus": cfg.Metrics.EnablePrometheus,
		})
	}

	// Setup the server with our mux that includes both proxy and metrics
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mainMux,
	}

	// Start the server in a goroutine
	go func() {
		logger.Info(fmt.Sprintf("Starting go-conductor on port %d", cfg.Port))
		logger.InfoWithFields(fmt.Sprintf("Configured to proxy requests to %d services with %d second timeout",
			len(cfg.Services), cfg.Timeout), map[string]interface{}{
			"services_count": len(cfg.Services),
			"timeout":        cfg.Timeout,
		})
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", err)
		}
	}()

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop
	logger.Info("Shutting down server...")
}
