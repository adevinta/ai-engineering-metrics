package pipeline

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/publisher"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
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
	Users      users.UserList              `yaml:"users"`
}

type Pipeline struct {
	Users      users.UsersList
	Collectors []collector.Collector
	Publishers []publisher.Publisher
}

func NewPipeline(cfg PipelineConfig) (Pipeline, error) {
	pipeline := Pipeline{}

	userList, err := users.NewUserList(cfg.Users)
	if err != nil {
		return Pipeline{}, fmt.Errorf("failed to create user list: %w", err)
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
	start = start.Truncate(24 * time.Hour)

	if start.After(time.Now()) {
		return fmt.Errorf("start time is in the future")
	}

	if end.Before(start) {
		return fmt.Errorf("end time is before start time")
	}

	for start.Before(end) {
		e := start.Add(24*time.Hour - time.Microsecond)
		if time.Now().Before(e) {
			log.Printf("end time is in the future, skip reporting %s\n", start.Format("2006-01-02"))
			break
		}
		fmt.Println("start", start, "end", e)
		if err := p.runOnce(ctx, start, e); err != nil {
			return fmt.Errorf("failed to run pipeline: %w", err)
		}
		start = start.Add(24 * time.Hour)
	}
	return nil
}

func (p *Pipeline) runOnce(ctx context.Context, start, end time.Time) error {
	log.Printf("Collecting metrics from %s to %s", start.Format(time.RFC3339), end.Format(time.RFC3339))

	// Collect metrics from all collectors
	allMetrics := make(map[collector.ToolUsage]collector.Metric)
	for _, c := range p.Collectors {
		log.Printf("Collecting from %s...", c.Name())
		metrics, err := c.Collect(ctx, start, end)
		if err != nil {
			log.Printf("Error collecting from %s: %v", c.Name(), err)
			continue
		}
		log.Printf("Collected %d metrics from %s", len(metrics), c.Name())
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

	for _, p := range p.Publishers {
		log.Printf("Publishing to %s...", p.Name())
		if err := p.Publish(ctx, start, end, allMetrics); err != nil {
			log.Printf("Error publishing to %s: %v", p.Name(), err)
			continue
		}
		log.Printf("Successfully published %d metrics to %s", len(allMetrics), p.Name())
	}

	return nil
}
