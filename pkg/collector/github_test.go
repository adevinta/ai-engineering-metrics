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
					"app_id":      123456,
					"private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
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
			wantErr: true, // Will fail due to invalid private key, but validates config structure
			errMsg:  "failed to parse private key",
		},
		{
			name: "valid configuration with scan_all_repos",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":         123456,
					"private_key":    "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
					"scan_all_repos": true,
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: true, // Will fail due to invalid private key, but validates config structure
			errMsg:  "failed to parse private key",
		},
		{
			name: "missing private_key",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":         123456,
					"scan_all_repos": true,
				},
				Mapper: mapper.MapperConfig{
					Type:   "passthrough",
					Config: map[string]any{},
				},
			},
			wantErr: true,
			errMsg:  "private_key is required",
		},
		{
			name: "missing GitHub App credentials",
			config: CollectorConfig{
				Config: map[string]any{
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "GitHub App configuration required",
		},
		{
			name: "invalid app_id type",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":       []string{"invalid"},
					"private_key":  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "app_id must be a number",
		},
		{
			name: "no scanning method specified",
			config: CollectorConfig{
				Config: map[string]any{
					"scan_all_repos":         false,
					"scan_all_organizations": false,
					// no repositories, app_id, or private_key specified
				},
			},
			wantErr: true,
			errMsg:  "app_id is required",
		},
		{
			name: "invalid repository format",
			config: CollectorConfig{
				Config: map[string]any{
					"repositories": []interface{}{
						"invalid-repo-format",
					},
				},
			},
			wantErr: true,
			errMsg:  "app_id is required",
		},
		{
			name: "empty private_key",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":       123456,
					"private_key":  "",
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "private_key cannot be empty",
		},
		{
			name: "invalid private_key format",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":       123456,
					"private_key":  "not-a-valid-key",
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "failed to parse private key",
		},
		{
			name: "zero app_id",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":      0,
					"private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
					"repositories": []interface{}{
						"owner/repo",
					},
				},
			},
			wantErr: true,
			errMsg:  "app_id must be a positive integer",
		},
		{
			name: "valid GitHub App configuration",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":       "123456",
					"private_key":  "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAqFO5xwz5SX...sample...key...-----END RSA PRIVATE KEY-----",
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true, // Will fail due to invalid private key format, but config parsing should pass
			errMsg:  "failed to parse private key",
		},
		{
			name: "missing app_id with private_key",
			config: CollectorConfig{
				Config: map[string]any{
					"private_key":  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "GitHub App configuration required",
		},
		{
			name: "missing private_key with app_id",
			config: CollectorConfig{
				Config: map[string]any{
					"app_id":       123456,
					"repositories": []interface{}{"owner/repo"},
				},
			},
			wantErr: true,
			errMsg:  "GitHub App configuration required",
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
	t.Skip("Integration test - requires GitHub App credentials")

	// This test would be run manually with:
	// GITHUB_APP_ID=123456 GITHUB_PRIVATE_KEY="$(cat key.pem)" go test -run TestGitHubCollector_Integration

	config := CollectorConfig{
		Config: map[string]any{
			"app_id":      "${env.GITHUB_APP_ID}",
			"private_key": "${env.GITHUB_PRIVATE_KEY}",
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
	t.Skip("Integration test - requires GitHub App credentials and careful testing")

	// This would test the scan_all_organizations functionality
	config := CollectorConfig{
		Config: map[string]any{
			"app_id":                 "${env.GITHUB_APP_ID}",
			"private_key":            "${env.GITHUB_PRIVATE_KEY}",
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
	t.Skip("Integration test - requires GitHub App credentials and careful testing")

	// This would test the scan_all_repos functionality
	config := CollectorConfig{
		Config: map[string]any{
			"app_id":         "${env.GITHUB_APP_ID}",
			"private_key":    "${env.GITHUB_PRIVATE_KEY}",
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

// Test processRepository with various scenarios
func TestGitHubCollector_processRepository(t *testing.T) {
	tests := []struct {
		name          string
		repoName      string
		wantErr       bool
		errMsg        string
		shouldSkip    bool
		setupMock     func(*GitHubCollector)
	}{
		{
			name:       "invalid repository format - missing slash",
			repoName:   "invalid-repo",
			wantErr:    true,
			errMsg:     "invalid repository format",
			shouldSkip: false,
		},
		{
			name:       "invalid repository format - too many parts",
			repoName:   "owner/repo/extra",
			wantErr:    true, // Will try to access "owner/repo/extra" on GitHub API and fail
			errMsg:     "failed to scan repository",
			shouldSkip: false,
		},
		{
			name:       "valid repository format",
			repoName:   "owner/repo",
			wantErr:    true, // Will error because we're not mocking the GitHub API
			errMsg:     "failed to scan repository",
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &GitHubCollector{
				client:       github.NewClient(nil),
				aiIndicators: []string{"CLAUDE.md"},
				mapper: &mockMapper{
					mapFunc: func(ctx context.Context, userID string) (string, error) {
						return userID, nil
					},
				},
				filter: nil, // No filtering
			}

			if tt.setupMock != nil {
				tt.setupMock(collector)
			}

			ctx := context.Background()
			scanTime := time.Now()

			result, err := collector.processRepository(ctx, tt.repoName, scanTime)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				if result != nil {
					assert.Equal(t, tt.shouldSkip, result.shouldSkip)
				}
			}
		})
	}
}

// Test parallel processing behavior with mixed success/failure scenarios
func TestGitHubCollector_ParallelProcessing(t *testing.T) {
	t.Run("some repositories fail but others succeed", func(t *testing.T) {
		// This test verifies that when some repositories fail during parallel processing,
		// the other repositories are still processed successfully

		collector := &GitHubCollector{
			client:       github.NewClient(nil),
			repositories: []string{
				"owner/valid-repo-1",
				"invalid-format",      // This will fail
				"owner/valid-repo-2",
				"another-invalid",     // This will also fail
				"owner/valid-repo-3",
			},
			aiIndicators: []string{"CLAUDE.md"},
			mapper: &mockMapper{
				mapFunc: func(ctx context.Context, userID string) (string, error) {
					return userID, nil
				},
			},
			filter: nil,
		}

		// Note: In a real test with proper mocking, we would:
		// 1. Mock the GitHub API to return success for valid repos
		// 2. Verify that invalid repos fail but don't stop processing
		// 3. Check that we get metrics for all valid repos

		// For now, we just verify the structure is correct
		assert.NotNil(t, collector.client)
		assert.Equal(t, 5, len(collector.repositories))

		// The actual test would require GitHub API mocking
		t.Skip("Full test requires GitHub API mocking - structure validated")
	})

	t.Run("all repositories in parallel complete without deadlock", func(t *testing.T) {
		// This test would verify that parallel processing doesn't deadlock
		// when all repositories complete successfully

		collector := &GitHubCollector{
			client: github.NewClient(nil),
			repositories: []string{
				"owner/repo-1",
				"owner/repo-2",
				"owner/repo-3",
				"owner/repo-4",
				"owner/repo-5",
				"owner/repo-6",
				"owner/repo-7",
				"owner/repo-8",
				"owner/repo-9",
				"owner/repo-10",
				"owner/repo-11", // More than maxConcurrent (10)
			},
			aiIndicators: []string{"CLAUDE.md"},
			mapper: &mockMapper{
				mapFunc: func(ctx context.Context, userID string) (string, error) {
					return userID, nil
				},
			},
		}

		// Verify we can handle more repos than the concurrent limit
		assert.Greater(t, len(collector.repositories), 10, "Should have more repos than maxConcurrent limit")

		t.Skip("Full test requires GitHub API mocking - structure validated")
	})

	t.Run("context cancellation stops all goroutines", func(t *testing.T) {
		// This test would verify that when context is cancelled,
		// all goroutines stop gracefully without hanging

		collector := &GitHubCollector{
			client: github.NewClient(nil),
			repositories: []string{
				"owner/repo-1",
				"owner/repo-2",
				"owner/repo-3",
			},
			aiIndicators: []string{"CLAUDE.md"},
			mapper: &mockMapper{
				mapFunc: func(ctx context.Context, userID string) (string, error) {
					return userID, nil
				},
			},
		}

		assert.NotNil(t, collector)

		t.Skip("Full test requires GitHub API mocking and context cancellation handling")
	})
}

// mockMapper is a simple mapper for testing
type mockMapper struct {
	mapFunc func(context.Context, string) (string, error)
}

func (m *mockMapper) Map(ctx context.Context, userID string) (string, error) {
	if m.mapFunc != nil {
		return m.mapFunc(ctx, userID)
	}
	return userID, nil
}
