package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 8081, "Port to listen on")
	serverName := flag.String("name", "default", "Server name")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] %s %s", *serverName, r.Method, r.URL.Path)

		// Log request headers
		for headerName, values := range r.Header {
			for _, value := range values {
				log.Printf("[%s] Header: %s = %s", *serverName, headerName, value)
			}
		}

		fmt.Fprintf(w, "Hello from %s server!\n", *serverName)
		fmt.Fprintf(w, "Path: %s\n", r.URL.Path)
		fmt.Fprintf(w, "Request received by %s service running on port %d\n", *serverName, *port)
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting mock server '%s' on %s", *serverName, addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
