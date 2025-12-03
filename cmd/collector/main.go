package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/pipeline"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	configPath = flag.String("config", "collector.yaml", "Path to configuration file")
	startTime  = flag.String("start", "", "Start time for metric collection (RFC3339 format). If not provided, it will be set to the start of previous day")
	utc        = flag.Bool("utc", true, "Use UTC timezone for the time range. If false, the local timezone will be used.")
	endTime    = flag.String("end", "", "End time for metric collection (RFC3339 format). If not provided, it will be set to the end of the previous day")
)

func main() {
	flag.Parse()

	// Initialize structured logging
	logging.InitLogger()

	if err := run(); err != nil {
		logrus.Fatalf("Error: %v", err)
	}
}

func run() error {
	logging.InitLogger(logging.WithLoggingFormatter(&logrus.JSONFormatter{}), logging.WithLoggingLevel(logrus.TraceLevel))
	ctx := logging.WithLoggingFields(context.Background(), logrus.Fields{
		"component":   "collector_main",
		"config_path": *configPath,
	})
	logger := logging.LoggerFromCtx(ctx)

	logger.Info("starting ai metrics collector")

	// Load configuration
	pipelines, err := pipeline.LoadPipelines(*configPath)
	if err != nil {
		logger.WithError(err).Error("failed to load config")
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.WithField("pipeline_count", len(pipelines)).Info("loaded pipelines")

	// Parse time range
	var start, end time.Time
	if *startTime != "" {
		if *utc {
			start, err = time.ParseInLocation(time.RFC3339, *startTime, time.UTC)
		} else {
			start, err = time.Parse(time.RFC3339, *startTime)
		}
		if err != nil {
			logger.WithError(err).WithField("start_time", *startTime).Error("invalid start time")
			return fmt.Errorf("invalid start time: %w", err)
		}
	} else {
		if *utc {
			start = time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
		} else {
			start = time.Now().Truncate(24 * time.Hour).Add(-24 * time.Hour)
		}
		logger.WithField("start_time", start).Info("using default start time (previous day)")
	}

	if *endTime != "" {
		if *utc {
			end, err = time.ParseInLocation(time.RFC3339, *endTime, time.UTC)
		} else {
			end, err = time.Parse(time.RFC3339, *endTime)
		}
		if err != nil {
			logger.WithError(err).WithField("end_time", *endTime).Error("invalid end time")
			return fmt.Errorf("invalid end time: %w", err)
		}
	} else {
		if *utc {
			end = time.Now().UTC().Truncate(24 * time.Hour).Add(-time.Microsecond)
		} else {
			end = time.Now().Truncate(24 * time.Hour).Add(-time.Microsecond)
		}
		logger.WithField("end_time", end).Info("using default end time (end of previous day)")
	}

	logger.WithFields(logrus.Fields{
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
		"utc":        *utc,
	}).Info("time range configured")

	// Run collection and publishing
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down gracefully...")
		cancel()
	}()

	logger.Info("starting pipeline execution")
	errGroup := errgroup.Group{}
	for i, pipeline := range pipelines {
		errGroup.Go(func() error {
			return pipeline.Run(logging.WithLoggingFields(ctx, logrus.Fields{
				"pipeline_index": i,
			}), start, end)
		})
	}

	if err := errGroup.Wait(); err != nil {
		logger.WithError(err).Error("failed to run pipelines")
		return fmt.Errorf("failed to run pipelines: %w", err)
	}

	logger.Info("ai metrics collector completed successfully")
	return nil
}
