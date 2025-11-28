package publisher

import (
	"context"
	"fmt"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
)

// GetDXPublisher publishes metrics to the GetDX platform
type GetDXPublisher struct {
	*dx.DatacloudAPIClient
	name     string
	userList users.UsersList
	tools    map[string]struct{}
}

func NewGetDXPublisher(cfg map[string]any, userList users.UsersList) (*GetDXPublisher, error) {
	apiKey, ok := cfg["api_token"].(string)
	if !ok {
		return nil, fmt.Errorf("api_token is not a string")
	}
	apiKey, err := lcel.ExpandEnv(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to expand api key: %w", err)
	}
	apiURL, ok := cfg["api_base_url"].(string)
	if !ok {
		return nil, fmt.Errorf("api_base_url is not a string")
	}
	tools, ok := cfg["tools"].([]any)
	if !ok {
		return nil, fmt.Errorf("tools is missing or not a list")
	}
	toolsMap := make(map[string]struct{})
	for _, tool := range tools {
		toolString, ok := tool.(string)
		if !ok {
			return nil, fmt.Errorf("tool is not a string")
		}
		toolsMap[toolString] = struct{}{}
	}
	dxClient, err := dx.NewDatacloudAPIClient(dx.WithDatacloudAPIKey(apiKey), dx.WithDatacloudAPIURL(apiURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create DX client: %w", err)
	}
	return &GetDXPublisher{
		DatacloudAPIClient: dxClient,
		userList:           userList,
		tools:              toolsMap,
	}, nil
}

// Name returns the publisher's name
func (p *GetDXPublisher) Name() string {
	return "getdx"
}

// Publish sends metrics to GetDX
func (p *GetDXPublisher) Publish(ctx context.Context, start, end time.Time, metrics map[collector.ToolUsage]collector.Metric) error {
	dxMetrics := make([]dx.DXAIMetric, 0)

	for key, metric := range metrics {
		dxMetrics = append(dxMetrics, newUsedMetric(formatDate(start), key.UserID, key.ToolName, metric.Metrics))
	}

	for tool := range p.tools {
		for _, userID := range p.userList.List() {
			if _, ok := metrics[collector.ToolUsage{UserID: userID, ToolName: tool}]; !ok {
				dxMetrics = append(dxMetrics, newUnusedMetric(formatDate(start), userID, tool))
			}
		}
	}
	// todo: paginate to limit request size
	_, err := p.PushAIMetrics(ctx, dx.DXAIMetrics{Data: dxMetrics})
	if err != nil {
		return fmt.Errorf("failed to push metrics: %w", err)
	}
	return nil
}

func newUsedMetric(date, userID, toolName string, metrics map[string]any) dx.DXAIMetric {
	return dx.DXAIMetric{
		Email:    userID,
		Date:     date,
		IsActive: true,
		Tool:     toolName,
		Metrics:  metrics,
	}
}

func newUnusedMetric(date, userID, toolName string) dx.DXAIMetric {
	return dx.DXAIMetric{
		Email:    userID,
		Date:     date,
		IsActive: false,
		Tool:     toolName,
		Metrics:  make(map[string]any),
	}
}

func formatDate(date time.Time) string {
	return date.Format("2006-01-02")
}
