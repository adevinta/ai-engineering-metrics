package types

import (
	"context"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
)

// Publisher defines the interface for publishing metrics to external platforms
type Publisher interface {
	// Publish sends metrics to the external platform
	Publish(ctx context.Context, metrics []collector.Metric) error
	// Name returns the publisher's name
	Name() string
}

// UserIDMapper defines the interface for mapping between different user ID formats
type UserIDMapper interface {
	// Map converts a user ID from one format to another
	Map(ctx context.Context, userID string) (string, error)
	// BatchMap converts multiple user IDs efficiently
	BatchMap(ctx context.Context, userIDs []string) (map[string]string, error)
}
