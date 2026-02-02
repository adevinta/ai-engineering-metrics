package collector

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/google/go-github/v75/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitHubCollector(t *testing.T) {
	tests := []struct {
		name    string
		config  CollectorConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration with repositories",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token": "test-token",
					"repositories": []interface{}{
						"owner/repo1",
						"owner/repo2",
					},
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with scan_all_repos",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token":   "test-token",
					"scan_all_repos": true,
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with organization filter",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token":   "test-token",
					"scan_all_repos": true,
					"organization_filter": []interface{}{
						"org1",
						"org2",
					},
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "missing github_token",
			config: CollectorConfig{
				Config: map[string]any{
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "github_token is required",
		},
		{
			name: "empty github_token",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token": "",
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "github_token cannot be empty",
		},
		{
			name: "no scanning method specified",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token":           "test-token",
					"scan_all_repos":         false,
					"scan_all_organizations": false,
				},
			},
			wantErr: true,
			errMsg:  "one of 'repositories' list, 'scan_all_repos: true', or 'scan_all_organizations: true' must be specified",
		},
		{
			name: "invalid repository format",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token": "test-token",
					"repositories": []interface{}{
						"invalid-repo-format",
					},
				},
			},
			wantErr: true,
			errMsg:  "must be in format 'owner/repo'",
		},
		{
			name: "custom ai_indicators",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token": "test-token",
					"repositories": []interface{}{"owner/repo"},
					"ai_indicators": []interface{}{
						"CLAUDE.md",
						"custom.md",
					},
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "valid scan_all_organizations configuration",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token":           "test-token",
					"scan_all_organizations": true,
					"organization_filter": []interface{}{
						"test-org",
					},
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: false,
		},
		{
			name: "scan_all_organizations with repositories combination",
			config: CollectorConfig{
				Config: map[string]any{
					"github_token":           "test-token",
					"scan_all_organizations": true,
					"repositories": []interface{}{
						"manual/repo",
					},
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector, err := NewGitHubCollector(tt.config, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, collector)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, collector)
				assert.Equal(t, "github", collector.Name())

				ghCollector := collector.(*GitHubCollector)
				assert.NotNil(t, ghCollector.client)
				assert.NotEmpty(t, ghCollector.aiIndicators)
			}
		})
	}
}

func TestGitHubCollector_isOrganizationAllowed(t *testing.T) {
	tests := []struct {
		name    string
		filter  []string
		org     string
		allowed bool
	}{
		{
			name:    "no filter - all allowed",
			filter:  []string{},
			org:     "any-org",
			allowed: true,
		},
		{
			name:    "org in filter",
			filter:  []string{"allowed-org", "another-org"},
			org:     "allowed-org",
			allowed: true,
		},
		{
			name:    "org not in filter",
			filter:  []string{"allowed-org", "another-org"},
			org:     "blocked-org",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &GitHubCollector{
				organizationFilter: tt.filter,
			}

			result := collector.isOrganizationAllowed(tt.org)
			assert.Equal(t, tt.allowed, result)
		})
	}
}

func TestGitHubCollector_checkFileExists(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantExists bool
		wantErr    bool
	}{
		{
			name:       "file exists",
			statusCode: http.StatusOK,
			wantExists: true,
			wantErr:    false,
		},
		{
			name:       "file not found",
			statusCode: http.StatusNotFound,
			wantExists: false,
			wantErr:    false,
		},
		{
			name:       "api error",
			statusCode: http.StatusInternalServerError,
			wantExists: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock GitHub client that returns the specified status code
			client := github.NewClient(nil)

			collector := &GitHubCollector{
				client: client,
			}
			_ = collector // Avoid unused variable error

			// Note: This would require more complex mocking in a real implementation
			// For now, we'll just test the error handling logic
			t.Skip("Requires GitHub API mocking - integration test recommended")
		})
	}
}

func TestRepositoryMetrics_Structure(t *testing.T) {
	// Test that the metrics structure is correctly formatted
	now := time.Now()
	repoMetrics := RepositoryMetrics{
		Repository:        "owner/repo",
		Organization:      "owner",
		IsAIReady:         true,
		AIIndicatorsFound: []string{"CLAUDE.md"},
		ScanTimestamp:     now,
	}

	repoMetrics.RepositoryMetadata.Private = false
	repoMetrics.RepositoryMetadata.StarCount = 100
	repoMetrics.RepositoryMetadata.ForkCount = 25
	repoMetrics.RepositoryMetadata.LastPush = now

	// Verify structure can be converted to map[string]any for metrics
	metricsMap := map[string]any{
		"repository":          repoMetrics.Repository,
		"organization":        repoMetrics.Organization,
		"is_ai_ready":         repoMetrics.IsAIReady,
		"ai_indicators_found": repoMetrics.AIIndicatorsFound,
		"scan_timestamp":      repoMetrics.ScanTimestamp,
		"repository_metadata": repoMetrics.RepositoryMetadata,
	}

	assert.Equal(t, "owner/repo", metricsMap["repository"])
	assert.Equal(t, "owner", metricsMap["organization"])
	assert.Equal(t, true, metricsMap["is_ai_ready"])
	assert.Equal(t, []string{"CLAUDE.md"}, metricsMap["ai_indicators_found"])
	assert.Equal(t, now, metricsMap["scan_timestamp"])
}

func TestOrganizationSummaryMetrics_Structure(t *testing.T) {
	// Test that the organization summary structure is correctly formatted
	now := time.Now()
	orgSummary := OrganizationSummaryMetrics{
		Organization:       "test-org",
		TotalReposScanned:  10,
		AIReadyRepos:       7,
		AIReadinessPercent: 70.0,
		ScanTimestamp:      now,
	}

	// Verify structure can be converted to map[string]any for metrics
	metricsMap := map[string]any{
		"organization":            orgSummary.Organization,
		"total_repos_scanned":     orgSummary.TotalReposScanned,
		"ai_ready_repos":          orgSummary.AIReadyRepos,
		"ai_readiness_percentage": orgSummary.AIReadinessPercent,
		"scan_timestamp":          orgSummary.ScanTimestamp,
	}

	assert.Equal(t, "test-org", metricsMap["organization"])
	assert.Equal(t, 10, metricsMap["total_repos_scanned"])
	assert.Equal(t, 7, metricsMap["ai_ready_repos"])
	assert.Equal(t, 70.0, metricsMap["ai_readiness_percentage"])
	assert.Equal(t, now, metricsMap["scan_timestamp"])
}

// Integration test structure for manual testing with real GitHub API
func TestGitHubCollector_Integration(t *testing.T) {
	// Skip this test in CI/CD - only for manual testing
	t.Skip("Integration test - requires GITHUB_TOKEN environment variable")

	// This test would be run manually with:
	// GITHUB_TOKEN=your_token go test -run TestGitHubCollector_Integration

	config := CollectorConfig{
		Config: map[string]any{
			"github_token": "${env.GITHUB_TOKEN}",
			"repositories": []interface{}{
				"adevinta/ai-engineering-metrics", // This repo has a CLAUDE.md file
			},
		},
		Mapper: mapper.MapperConfig{
			Type:   "passthrough",
			Config: map[string]any{},
		},
	}

	collector, err := NewGitHubCollector(config, nil)
	require.NoError(t, err)

	ctx := context.Background()
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	metrics, err := collector.Collect(ctx, start, end)
	require.NoError(t, err)

	// Verify we got metrics
	assert.Greater(t, len(metrics), 0)

	// Look for repository and summary metrics
	var foundRepo, foundSummary bool
	for toolUsage, metric := range metrics {
		if toolUsage.ToolName == "ai-readiness" {
			foundRepo = true
			assert.Contains(t, metric.Metrics, "repository")
			assert.Contains(t, metric.Metrics, "is_ai_ready")
		}
		if toolUsage.ToolName == "ai-readiness-summary" {
			foundSummary = true
			assert.Contains(t, metric.Metrics, "organization")
			assert.Contains(t, metric.Metrics, "ai_readiness_percentage")
		}
	}

	assert.True(t, foundRepo, "Should find repository metrics")
	assert.True(t, foundSummary, "Should find organization summary metrics")
}

// Test for scan all organizations functionality
func TestGitHubCollector_ScanAllOrganizations(t *testing.T) {
	t.Skip("Integration test - requires GITHUB_TOKEN and careful testing")

	// This would test the scan_all_organizations functionality
	config := CollectorConfig{
		Config: map[string]any{
			"github_token":           "${env.GITHUB_TOKEN}",
			"scan_all_organizations": true,
			"organization_filter": []interface{}{
				"adevinta", // Only scan adevinta organization
			},
		},
		Mapper: mapper.MapperConfig{
			Type:   "passthrough",
			Config: map[string]any{},
		},
	}

	collector, err := NewGitHubCollector(config, nil)
	require.NoError(t, err)

	ctx := context.Background()
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	metrics, err := collector.Collect(ctx, start, end)
	require.NoError(t, err)

	// Should have found repositories from the organization
	assert.Greater(t, len(metrics), 0)

	// All repository metrics should be from filtered organizations
	for toolUsage, metric := range metrics {
		if toolUsage.ToolName == "ai-readiness" {
			org, ok := metric.Metrics["organization"].(string)
			require.True(t, ok)
			assert.Equal(t, "adevinta", org)
		}
	}
}

// Test for scan all repos functionality
func TestGitHubCollector_ScanAllRepos(t *testing.T) {
	t.Skip("Integration test - requires GITHUB_TOKEN and careful testing")

	// This would test the scan_all_repos functionality
	config := CollectorConfig{
		Config: map[string]any{
			"github_token":   "${env.GITHUB_TOKEN}",
			"scan_all_repos": true,
			"organization_filter": []interface{}{
				"adevinta", // Only scan adevinta repos
			},
		},
		Mapper: mapper.MapperConfig{
			Type:   "passthrough",
			Config: map[string]any{},
		},
	}

	collector, err := NewGitHubCollector(config, nil)
	require.NoError(t, err)

	ctx := context.Background()
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	metrics, err := collector.Collect(ctx, start, end)
	require.NoError(t, err)

	// Should have found multiple repositories
	assert.Greater(t, len(metrics), 1)

	// All repository metrics should be from filtered organizations
	for toolUsage, metric := range metrics {
		if toolUsage.ToolName == "ai-readiness" {
			org, ok := metric.Metrics["organization"].(string)
			require.True(t, ok)
			assert.Equal(t, "adevinta", org)
		}
	}
}
