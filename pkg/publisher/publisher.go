package publisher

import (
	"context"
	"fmt"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
)

// PublisherConfig represents configuration for a single publisher
type PublisherConfig struct {
	Name    string         `yaml:"name"`
	Type    string         `yaml:"type"`
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
}

// Publisher defines the interface for publishing metrics to external platforms
type Publisher interface {
	// Publish sends metrics to the external platform
	Publish(ctx context.Context, start, end time.Time, metrics map[collector.ToolUsage]collector.Metric) error
	// Name returns the publisher's name
	Name() string
}

func NewPublisher(cfg PublisherConfig, userList users.UsersList) (Publisher, error) {
	switch cfg.Type {
	case "getdx":
		p, err := NewGetDXPublisher(cfg.Config, userList)
		if err != nil {
			return nil, fmt.Errorf("failed to create publisher: %w", err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unknown publisher type: %s", cfg.Type)
	}
}
