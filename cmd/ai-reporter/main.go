package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/pipeline"
	"golang.org/x/sync/errgroup"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
	startTime  = flag.String("start", "", "Start time for metric collection (RFC3339 format). If not provided, it will be set to the start of previous day")
	endTime    = flag.String("end", "", "End time for metric collection (RFC3339 format). If not provided, it will be set to the end of the previous day")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	// Load configuration
	pipelines, err := pipeline.LoadPipelines(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Parse time range
	var start, end time.Time
	if *startTime != "" {
		start, err = time.Parse(time.RFC3339, *startTime)
		if err != nil {
			return fmt.Errorf("invalid start time: %w", err)
		}
	} else {
		start = time.Now().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	}

	if *endTime != "" {
		end, err = time.Parse(time.RFC3339, *endTime)
		if err != nil {
			return fmt.Errorf("invalid end time: %w", err)
		}
	} else {
		end = time.Now().Truncate(24 * time.Hour).Add(-time.Microsecond)
	}

	// Run collection and publishing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	errGroup := errgroup.Group{}
	for _, pipeline := range pipelines {
		errGroup.Go(func() error {
			return pipeline.Run(ctx, start, end)
		})
	}

	if err := errGroup.Wait(); err != nil {
		return fmt.Errorf("failed to run pipelines: %w", err)
	}

	return nil
}
