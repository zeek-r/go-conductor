package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/zeek-r/go-conductor/internal/config"
	"github.com/zeek-r/go-conductor/internal/proxy"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create proxy conductor
	conductor := proxy.NewConductor(cfg)

	// Setup the server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: conductor,
	}

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting go-conductor on port %d", cfg.Port)
		log.Printf("Configured to proxy requests to %d services with %d second timeout",
			len(cfg.Services), cfg.Timeout)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")
}
