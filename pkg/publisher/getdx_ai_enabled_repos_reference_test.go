package publisher

import (
	"testing"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/collector"
	"github.com/stretchr/testify/assert"
)

func TestReference_Sanitization(t *testing.T) {
	publisher := &GetDXAIEnabledReposPublisher{
		tools: map[string]struct{}{
			"ai-readiness":         {},
			"ai-readiness-summary": {},
		},
	}

	tests := []struct {
		name         string
		orgName      string
		expectedRef  string
		toolName     string
	}{
		{
			name:        "organization with hyphens",
			orgName:     "claude-code-actions",
			expectedRef: "ai-readiness-summary-claude_code_actions",
			toolName:    "ai-readiness-summary",
		},
		{
			name:        "organization with dots",
			orgName:     "my.org.com",
			expectedRef: "ai-readiness-summary-my_org_com",
			toolName:    "ai-readiness-summary",
		},
		{
			name:        "repository reference with special chars",
			orgName:     "test-org",
			expectedRef: "ai-readiness-test_org",
			toolName:    "ai-readiness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var metrics map[string]any
			if tt.toolName == "ai-readiness-summary" {
				metrics = map[string]any{
					"organization":            tt.orgName,
					"total_repos_scanned":     10,
					"ai_ready_repos":          5,
					"ai_readiness_percentage": 50.0,
				}
			} else {
				metrics = map[string]any{
					"repository":    "test/repo",
					"organization":  tt.orgName,
					"is_ai_ready":   true,
					"files_found":   []string{"CLAUDE.md"},
				}
			}

			toolUsage := collector.ToolUsage{
				UserID:   "test@example.com",
				ToolName: tt.toolName,
			}

			metric := collector.Metric{
				UserID:   "test@example.com",
				ToolName: tt.toolName,
				Metrics:  metrics,
			}

			customMetrics, err := publisher.convertToCustomMetrics(toolUsage, metric, time.Now())
			assert.NoError(t, err)
			assert.NotEmpty(t, customMetrics)

			// Check that at least one metric has the expected reference
			found := false
			for _, cm := range customMetrics {
				if cm.Reference == tt.expectedRef {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected reference %s not found in custom metrics", tt.expectedRef)
		})
	}
}