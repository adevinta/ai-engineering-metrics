package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
)

// GetDXPublisher publishes metrics to the GetDX platform
type GetDXPublisher struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
	name       string
	userList   users.UsersList
}

func NewGetDXPublisher(cfg map[string]any, userList users.UsersList) (*GetDXPublisher, error) {
	apiKey, ok := cfg["api_token"].(string)
	if !ok {
		return nil, fmt.Errorf("api_token is not a string")
	}
	apiURL, ok := cfg["api_base_url"].(string)
	if !ok {
		return nil, fmt.Errorf("api_base_url is not a string")
	}
	return &GetDXPublisher{
		apiKey: apiKey,
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userList: userList,
	}, nil
}

// GetDXMetric represents a metric in GetDX format
type GetDXMetric struct {
	Email    string         `json:"email"`
	Date     string         `json:"date"`
	IsActive bool           `json:"is_active"`
	Tool     string         `json:"tool"`
	Metrics  map[string]any `json:"metrics"`
}

// GetDXBatchRequest represents a batch of metrics to send to GetDX
type GetDXBatchRequest struct {
	Metrics []GetDXMetric `json:"metrics"`
}

// Name returns the publisher's name
func (p *GetDXPublisher) Name() string {
	return "getdx"
}

// Publish sends metrics to GetDX
func (p *GetDXPublisher) Publish(ctx context.Context, start, end time.Time, metrics map[collector.ToolUsage]collector.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	tools := map[string]struct{}{}

	for key, metric := range metrics {
		tools[metric.ToolName] = struct{}{}
		if err := p.publishMetric(ctx, newUsedMetric(formatDate(start), key.UserID, key.ToolName, metric.Metrics)); err != nil {
			return fmt.Errorf("failed to publish unused metric: %w", err)
		}
	}

	for tool := range tools {
		for _, userID := range p.userList.List() {
			if _, ok := metrics[collector.ToolUsage{UserID: userID, ToolName: tool}]; !ok {
				if err := p.publishMetric(ctx, newUnusedMetric(formatDate(start), userID, tool)); err != nil {
					return fmt.Errorf("failed to publish unused metric: %w", err)
				}
			}
		}
	}
	return nil
}

func (p *GetDXPublisher) publishMetric(ctx context.Context, metric GetDXMetric) error {
	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/api/aiToolMetrics.push", strings.TrimSuffix(p.apiURL, "/")), bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

func newUsedMetric(date, userID, toolName string, metrics map[string]any) GetDXMetric {
	return GetDXMetric{
		Email:    userID,
		Date:     date,
		IsActive: true,
		Tool:     toolName,
		Metrics:  metrics,
	}
}

func newUnusedMetric(date, userID, toolName string) GetDXMetric {
	return GetDXMetric{
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
