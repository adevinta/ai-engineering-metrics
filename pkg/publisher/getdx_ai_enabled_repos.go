package publisher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/sirupsen/logrus"
)

// GetDXAIEnabledReposPublisher publishes ai-readiness metrics as GetDX custom metrics
type GetDXAIEnabledReposPublisher struct {
	*dx.DatacloudAPIClient
	name     string
	userList users.UsersList
	tools    map[string]struct{}
}

func NewGetDXAIEnabledReposPublisher(cfg map[string]any, userList users.UsersList) (*GetDXAIEnabledReposPublisher, error) {
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
	return &GetDXAIEnabledReposPublisher{
		DatacloudAPIClient: dxClient,
		userList:           userList,
		tools:              toolsMap,
	}, nil
}

// Name returns the publisher's name
func (p *GetDXAIEnabledReposPublisher) Name() string {
	return "getdx-ai-enabled-repos"
}

// Publish sends ai-readiness metrics as custom metrics to GetDX
func (p *GetDXAIEnabledReposPublisher) Publish(ctx context.Context, start, end time.Time, metrics map[collector.ToolUsage]collector.Metric) error {
	ctx = logging.WithLoggingFields(ctx, logrus.Fields{
		"component":        "getdx_ai_enabled_repos_publisher",
		"publisher":        "getdx-ai-enabled-repos",
		"start_date":       formatDate(start),
		"end_date":         formatDate(end),
		"input_metrics":    len(metrics),
		"configured_tools": len(p.tools),
	})
	logger := logging.LoggerFromCtx(ctx)

	logger.Info("starting dx custom metric publishing")

	publishedCount := 0
	errors := make([]error, 0)

	for key, metric := range metrics {
		// Only process ai-readiness metrics
		if !strings.HasPrefix(key.ToolName, "ai-readiness") {
			continue
		}

		// Check if tool is configured to be published
		if _, ok := p.tools[key.ToolName]; !ok {
			continue
		}

		customMetrics, err := p.convertToCustomMetrics(key, metric, start)
		if err != nil {
			logger.WithError(err).WithFields(logrus.Fields{
				"user_id":   key.UserID,
				"tool_name": key.ToolName,
			}).Warn("failed to convert metric to custom format")
			errors = append(errors, err)
			continue
		}

		// Send each custom metric
		for _, customMetric := range customMetrics {
			publishStart := time.Now()
			_, err := p.PushCustomMetric(ctx, customMetric)
			duration := time.Since(publishStart)

			if err != nil {
				logger.WithError(err).WithFields(logrus.Fields{
					"duration_ms": duration.Milliseconds(),
					"reference":   customMetric.Reference,
					"key":         customMetric.Key,
				}).Error("failed to push custom metric to dx")
				errors = append(errors, err)
				continue
			}

			publishedCount++
			logger.WithFields(logrus.Fields{
				"duration_ms": duration.Milliseconds(),
				"reference":   customMetric.Reference,
				"key":         customMetric.Key,
				"value":       customMetric.Value,
			}).Debug("successfully pushed custom metric")
		}
	}

	logger.WithFields(logrus.Fields{
		"published_metrics": publishedCount,
		"errors":           len(errors),
	}).Info("completed dx custom metric publishing")

	if len(errors) > 0 {
		return fmt.Errorf("failed to publish %d metrics: %v", len(errors), errors[0])
	}

	return nil
}

// convertToCustomMetrics converts ai-readiness metrics to GetDX custom metric format
func (p *GetDXAIEnabledReposPublisher) convertToCustomMetrics(key collector.ToolUsage, metric collector.Metric, timestamp time.Time) ([]dx.DXCustomMetric, error) {
	timestampStr := timestamp.Format(time.RFC3339)
	var customMetrics []dx.DXCustomMetric

	switch key.ToolName {
	case "ai-readiness":
		// Individual repository metrics
		repository, ok := metric.Metrics["repository"].(string)
		if !ok {
			return nil, fmt.Errorf("repository field missing or not a string")
		}
		organization, ok := metric.Metrics["organization"].(string)
		if !ok {
			return nil, fmt.Errorf("organization field missing or not a string")
		}
		isAIReady, ok := metric.Metrics["is_ai_ready"].(bool)
		if !ok {
			return nil, fmt.Errorf("is_ai_ready field missing or not a boolean")
		}

		// Convert boolean to float64 (1.0 for true, 0.0 for false)
		readinessValue := 0.0
		if isAIReady {
			readinessValue = 1.0
		}

		// Create the custom metric
		customMetrics = append(customMetrics, dx.DXCustomMetric{
			Reference: fmt.Sprintf("ai-readiness-%s", sanitizeKey(organization)),
			Key:       fmt.Sprintf("repository_ai_ready_%s", sanitizeKey(repository)),
			Value:     readinessValue,
			Metadata: map[string]any{
				"source":       "ai-metrics-collector",
				"repository":   repository,
				"organization": organization,
			},
			Timestamp: timestampStr,
		})

	case "ai-readiness-summary":
		// Organizational summary metrics
		organization, ok := metric.Metrics["organization"].(string)
		if !ok {
			return nil, fmt.Errorf("organization field missing or not a string")
		}

		// Total repositories scanned
		if totalRepos, ok := metric.Metrics["total_repos_scanned"].(int); ok {
			customMetrics = append(customMetrics, dx.DXCustomMetric{
				Reference: fmt.Sprintf("ai-readiness-summary-%s", sanitizeKey(organization)),
				Key:       "total_repos_scanned",
				Value:     float64(totalRepos),
				Metadata: map[string]any{
					"source":       "ai-metrics-collector",
					"organization": organization,
					"metric_type":  "count",
				},
				Timestamp: timestampStr,
			})
		}

		// AI ready repositories count
		if aiReadyRepos, ok := metric.Metrics["ai_ready_repos"].(int); ok {
			customMetrics = append(customMetrics, dx.DXCustomMetric{
				Reference: fmt.Sprintf("ai-readiness-summary-%s", sanitizeKey(organization)),
				Key:       "ai_ready_repos",
				Value:     float64(aiReadyRepos),
				Metadata: map[string]any{
					"source":       "ai-metrics-collector",
					"organization": organization,
					"metric_type":  "count",
				},
				Timestamp: timestampStr,
			})
		}

		// AI readiness percentage
		if readinessPercent, ok := metric.Metrics["ai_readiness_percentage"].(float64); ok {
			customMetrics = append(customMetrics, dx.DXCustomMetric{
				Reference: fmt.Sprintf("ai-readiness-summary-%s", sanitizeKey(organization)),
				Key:       "ai_readiness_percentage",
				Value:     readinessPercent,
				Metadata: map[string]any{
					"source":       "ai-metrics-collector",
					"organization": organization,
					"metric_type":  "percentage",
				},
				Timestamp: timestampStr,
			})
		}

	default:
		return nil, fmt.Errorf("unsupported tool name: %s", key.ToolName)
	}

	return customMetrics, nil
}

// sanitizeKey converts repository names to valid metric keys
func sanitizeKey(repository string) string {
	// Replace special characters with underscores
	result := strings.ReplaceAll(repository, "/", "_")
	result = strings.ReplaceAll(result, "-", "_")
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ToLower(result)
	return result
}

