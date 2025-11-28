package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/adevinta/ai-engineering-metrics/pkg/webhook"
)

func main() {
	var (
		configFile = flag.String("config", "webhook.yaml", "Path to webhook configuration file")
		port       = flag.String("port", "8080", "Port to serve on")
	)
	flag.Parse()

	// Load webhook configuration
	handler, err := webhook.NewHandler(webhook.FromConfigFile(*configFile))
	if err != nil {
		log.Fatalf("Failed to create webhook handler: %v", err)
	}

	// Start HTTP server
	addr := ":" + *port
	fmt.Printf("Starting webhook server on %s\n", addr)
	fmt.Printf("Using config file: %s\n", *configFile)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
