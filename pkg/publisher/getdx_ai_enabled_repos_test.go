package publisher

import (
	"testing"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/stretchr/testify/assert"
)

func TestNewGetDXAIEnabledReposPublisher(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]any
		expectError bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"api_token":    "test-token",
				"api_base_url": "https://test.getdx.net",
				"tools":        []any{"ai-readiness", "ai-readiness-summary"},
			},
			expectError: false,
		},
		{
			name: "missing api_token",
			config: map[string]any{
				"api_base_url": "https://test.getdx.net",
				"tools":        []any{"ai-readiness"},
			},
			expectError: true,
		},
		{
			name: "missing api_base_url",
			config: map[string]any{
				"api_token": "test-token",
				"tools":     []any{"ai-readiness"},
			},
			expectError: true,
		},
		{
			name: "missing tools",
			config: map[string]any{
				"api_token":    "test-token",
				"api_base_url": "https://test.getdx.net",
			},
			expectError: true,
		},
	}

	userList := &users.StaticUserFilter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGetDXAIEnabledReposPublisher(tt.config, userList)
			if (err != nil) != tt.expectError {
				t.Errorf("NewGetDXAIEnabledReposPublisher() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestConvertToCustomMetrics(t *testing.T) {
	publisher := &GetDXAIEnabledReposPublisher{}
	timestamp := time.Date(2026, 2, 3, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		toolUsage      collector.ToolUsage
		metric         collector.Metric
		expectedCount  int
		expectedError  bool
		expectedValues []float64
	}{
		{
			name: "ai-readiness metric",
			toolUsage: collector.ToolUsage{
				UserID:   "user@example.com",
				ToolName: "ai-readiness",
			},
			metric: collector.Metric{
				UserID:   "user@example.com",
				ToolName: "ai-readiness",
				Metrics: map[string]any{
					"repository":           "org/repo1",
					"organization":         "testorg",
					"is_ai_ready":          true,
					"ai_indicators_found":  []string{"CLAUDE.md"},
				},
			},
			expectedCount:  1,
			expectedError:  false,
			expectedValues: []float64{1.0}, // true converts to 1.0
		},
		{
			name: "ai-readiness metric - not ready",
			toolUsage: collector.ToolUsage{
				UserID:   "user@example.com",
				ToolName: "ai-readiness",
			},
			metric: collector.Metric{
				UserID:   "user@example.com",
				ToolName: "ai-readiness",
				Metrics: map[string]any{
					"repository":          "org/repo2",
					"organization":        "testorg",
					"is_ai_ready":         false,
					"ai_indicators_found": []string{},
				},
			},
			expectedCount:  1,
			expectedError:  false,
			expectedValues: []float64{0.0}, // false converts to 0.0
		},
		{
			name: "ai-readiness-summary metric",
			toolUsage: collector.ToolUsage{
				UserID:   "org@example.com",
				ToolName: "ai-readiness-summary",
			},
			metric: collector.Metric{
				UserID:   "org@example.com",
				ToolName: "ai-readiness-summary",
				Metrics: map[string]any{
					"organization":            "testorg",
					"total_repos_scanned":     100,
					"ai_ready_repos":          75,
					"ai_readiness_percentage": 75.0,
				},
			},
			expectedCount:  3,
			expectedError:  false,
			expectedValues: []float64{100.0, 75.0, 75.0},
		},
		{
			name: "invalid tool name",
			toolUsage: collector.ToolUsage{
				UserID:   "user@example.com",
				ToolName: "invalid-tool",
			},
			metric: collector.Metric{
				UserID:   "user@example.com",
				ToolName: "invalid-tool",
				Metrics:  map[string]any{},
			},
			expectedCount: 0,
			expectedError: true,
		},
		{
			name: "ai-readiness-summary metric with special chars in org name",
			toolUsage: collector.ToolUsage{
				UserID:   "special-org@example.com",
				ToolName: "ai-readiness-summary",
			},
			metric: collector.Metric{
				UserID:   "special-org@example.com",
				ToolName: "ai-readiness-summary",
				Metrics: map[string]any{
					"organization":            "claude-code-actions", // Org name with hyphens
					"total_repos_scanned":     50,
					"ai_ready_repos":          25,
					"ai_readiness_percentage": 50.0,
				},
			},
			expectedCount:  3,
			expectedError:  false,
			expectedValues: []float64{50.0, 25.0, 50.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customMetrics, err := publisher.convertToCustomMetrics(tt.toolUsage, tt.metric, timestamp)

			if (err != nil) != tt.expectedError {
				t.Errorf("convertToCustomMetrics() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			if err == nil {
				assert.Equal(t, tt.expectedCount, len(customMetrics))

				if len(customMetrics) > 0 && len(tt.expectedValues) > 0 {
					for i, expectedValue := range tt.expectedValues {
						if i < len(customMetrics) {
							assert.Equal(t, expectedValue, customMetrics[i].Value)
							assert.Equal(t, timestamp.Format(time.RFC3339), customMetrics[i].Timestamp)
							assert.Contains(t, customMetrics[i].Metadata, "source")
							assert.Equal(t, "ai-metrics-collector", customMetrics[i].Metadata["source"])
							// Verify user_id is not included
							assert.NotContains(t, customMetrics[i].Metadata, "user_id")
						}
					}
				}
			}
		})
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"org/repo-name", "org_repo_name"},
		{"My-Repo.Test", "my_repo_test"},
		{"simple", "simple"},
		{"org/sub/repo-with.dots", "org_sub_repo_with_dots"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDXAIEnabledReposPublisher_Name(t *testing.T) {
	publisher := &GetDXAIEnabledReposPublisher{}
	assert.Equal(t, "getdx-ai-enabled-repos", publisher.Name())
}