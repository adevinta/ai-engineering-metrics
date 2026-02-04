package pipeline

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/publisher"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert/yaml"
)

// Config represents the application configuration
type Config struct {
	Pipelines []PipelineConfig `yaml:"pipelines"`
}

func LoadPipelines(path string) ([]Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return NewPipelines(cfg)
}

func NewPipelines(cfg Config) ([]Pipeline, error) {
	pipelines := make([]Pipeline, 0, len(cfg.Pipelines))
	for _, pipelineCfg := range cfg.Pipelines {
		pipeline, err := NewPipeline(pipelineCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create pipeline: %w", err)
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines, nil
}

type PipelineConfig struct {
	Collectors []collector.CollectorConfig `yaml:"collectors"`
	Publishers []publisher.PublisherConfig `yaml:"publishers"`
	Users      *users.UserList             `yaml:"users,omitempty"`
}

type Pipeline struct {
	Users      users.UsersList
	Collectors []collector.Collector
	Publishers []publisher.Publisher
}

func NewPipeline(cfg PipelineConfig) (Pipeline, error) {
	pipeline := Pipeline{}

	var userList users.UsersList
	if cfg.Users != nil {
		var err error
		userList, err = users.NewUserList(*cfg.Users)
		if err != nil {
			return Pipeline{}, fmt.Errorf("failed to create user list: %w", err)
		}
	} else {
		// Default to allow all users when no user filtering is specified
		userList = users.NewAllowAllUserFilter()
	}
	pipeline.Users = userList

	for _, collectorCfg := range cfg.Collectors {
		collector, err := collector.NewCollector(collectorCfg, userList)
		if err != nil {
			return Pipeline{}, fmt.Errorf("failed to create collector: %w", err)
		}
		pipeline.Collectors = append(pipeline.Collectors, collector)
	}

	for _, publisherCfg := range cfg.Publishers {
		publisher, err := publisher.NewPublisher(publisherCfg, userList)
		if err != nil {
			return Pipeline{}, fmt.Errorf("failed to create publisher: %w", err)
		}
		pipeline.Publishers = append(pipeline.Publishers, publisher)
	}

	return pipeline, nil
}

func (p *Pipeline) Run(ctx context.Context, start, end time.Time) error {
	// Add pipeline context for logging
	ctx = logging.WithLoggingFields(ctx, logrus.Fields{
		"component": "pipeline",
		"operation": "run",
	})
	logger := logging.LoggerFromCtx(ctx)

	start = start.Truncate(24 * time.Hour)

	if start.After(time.Now()) {
		logger.WithField("start_time", start).Error("start time is in the future")
		return fmt.Errorf("start time is in the future")
	}

	if end.Before(start) {
		logger.WithFields(logrus.Fields{
			"start_time": start,
			"end_time":   end,
		}).Error("end time is before start time")
		return fmt.Errorf("end time is before start time")
	}

	logger.WithFields(logrus.Fields{
		"start_time": start,
		"end_time":   end,
	}).Info("pipeline started")

	for start.Before(end) {
		e := start.Add(24*time.Hour - time.Microsecond)
		if time.Now().Before(e) {
			logger.WithField("date", start.Format("2006-01-02")).Info("end time is in the future, skip reporting")
			break
		}

		dayLogger := logger.WithFields(logrus.Fields{
			"day_start": start,
			"day_end":   e,
		})
		dayLogger.Info("processing day")

		if err := p.runOnce(ctx, start, e); err != nil {
			dayLogger.WithError(err).Error("failed to run pipeline for day")
			return fmt.Errorf("failed to run pipeline: %w", err)
		}
		start = start.Add(24 * time.Hour)
	}

	logger.Info("pipeline completed successfully")
	return nil
}

func (p *Pipeline) runOnce(ctx context.Context, start, end time.Time) error {
	ctx = logging.WithLoggingField(ctx, "operation", "runOnce")
	logger := logging.LoggerFromCtx(ctx)

	logger.WithFields(logrus.Fields{
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
	}).Info("collecting metrics")

	// Collect metrics from all collectors
	allMetrics := make(map[collector.ToolUsage]collector.Metric)
	errors := make([]error, 0)

	for _, c := range p.Collectors {
		ctx = logging.WithLoggingField(ctx, "collector", c.Name())
		logger = logging.LoggerFromCtx(ctx)
		logger.Info("starting collection")
		collectorStart := time.Now()

		metrics, err := c.Collect(ctx, start, end)
		duration := time.Since(collectorStart)

		if err != nil {
			logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("collection failed")
			errors = append(errors, err)
			continue
		}

		logger.WithFields(logrus.Fields{
			"metric_count": len(metrics),
			"duration_ms":  duration.Milliseconds(),
		}).Info("collection completed")

		// Merge metrics
		for key, metric := range metrics {
			m, ok := allMetrics[key]
			if !ok {
				m = metric
			}
			for k, v := range metric.Metrics {
				m.Metrics[k] = v
			}
			allMetrics[key] = m
		}
	}

	// Publish metrics to all publishers
	for _, pub := range p.Publishers {
		ctx = logging.WithLoggingFields(ctx, logrus.Fields{
			"publisher":    pub.Name(),
			"metric_count": len(allMetrics),
		})
		logger = logging.LoggerFromCtx(ctx)
		logger.Info("starting publishing")
		publishStart := time.Now()

		if err := pub.Publish(ctx, start, end, allMetrics); err != nil {
			duration := time.Since(publishStart)
			logger.WithError(err).WithField("duration_ms", duration.Milliseconds()).Error("publishing failed")
			errors = append(errors, err)
			continue
		}

		duration := time.Since(publishStart)
		logger.WithField("duration_ms", duration.Milliseconds()).Info("publishing completed")
	}

	if len(errors) > 0 {
		logger.WithField("error_count", len(errors)).Error("pipeline completed with errors")
		return fmt.Errorf("failed to collect metrics from some collectors: %v", errors)
	}

	logger.Info("pipeline run completed successfully")
	return nil
}
