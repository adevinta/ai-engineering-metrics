package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/google/go-github/v75/github"
	"github.com/sirupsen/logrus"
)

// GitHubCollector collects AI readiness metrics from GitHub repositories
type GitHubCollector struct {
	client               *github.Client
	repositories         []string
	aiIndicators         []string
	scanAllRepos         bool
	scanAllOrganizations bool     // New: scan all organizations user has access to
	organizationFilter   []string // Optional: filter organizations when scanning
	mapper               mapper.UserIDMapper
	filter               users.UsersList
	// GitHub App authentication fields
	githubApp     *GitHubAppAuth  // GitHub App authentication handler
	installations []*Installation // GitHub App installations cache
}

var _ Collector = (*GitHubCollector)(nil)

// RepositoryMetrics holds AI readiness data for a single repository
type RepositoryMetrics struct {
	Repository         string    `json:"repository"`
	Organization       string    `json:"organization"`
	IsAIReady          bool      `json:"is_ai_ready"`
	AIIndicatorsFound  []string  `json:"ai_indicators_found"`
	ScanTimestamp      time.Time `json:"scan_timestamp"`
	RepositoryMetadata struct {
		Private   bool      `json:"private"`
		Language  *string   `json:"language"`
		LastPush  time.Time `json:"last_push"`
		StarCount int       `json:"star_count"`
		ForkCount int       `json:"fork_count"`
	} `json:"repository_metadata"`
}

// OrganizationSummaryMetrics holds aggregated AI readiness data for an organization
type OrganizationSummaryMetrics struct {
	Organization       string    `json:"organization"`
	TotalReposScanned  int       `json:"total_repos_scanned"`
	AIReadyRepos       int       `json:"ai_ready_repos"`
	AIReadinessPercent float64   `json:"ai_readiness_percentage"`
	ScanTimestamp      time.Time `json:"scan_timestamp"`
}

// NewGitHubCollector creates a new GitHub collector
func NewGitHubCollector(cfg CollectorConfig, userList users.UsersList) (Collector, error) {
	// GitHub App authentication is required
	if err := validateGitHubAppConfig(cfg.Config); err != nil {
		return nil, fmt.Errorf("GitHub App configuration required: %w", err)
	}

	// Extract GitHub base URL (optional, defaults to public GitHub)
	var baseURL string
	if baseURLRaw, ok := cfg.Config["github_base_url"].(string); ok {
		expandedBaseURL, err := lcel.ExpandEnv(baseURLRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to expand github_base_url: %w", err)
		}
		baseURL = expandedBaseURL
	}

	// Extract app ID
	appIDRaw := cfg.Config["app_id"]
	var appID int64
	switch v := appIDRaw.(type) {
	case string:
		// Expand environment variables in the string
		expandedAppID, err := lcel.ExpandEnv(v)
		if err != nil {
			return nil, fmt.Errorf("failed to expand app_id: %w", err)
		}
		parsed, err := strconv.ParseInt(expandedAppID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("app_id must be a valid integer: %w", err)
		}
		appID = parsed
	case int:
		appID = int64(v)
	case int64:
		appID = v
	case float64:
		appID = int64(v)
	default:
		return nil, fmt.Errorf("app_id must be a number")
	}

	// Extract and expand private key
	privateKeyRaw, ok := cfg.Config["private_key"].(string)
	if !ok {
		return nil, fmt.Errorf("private_key must be a string")
	}

	privateKey, err := lcel.ExpandEnv(privateKeyRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to expand private_key: %w", err)
	}

	if privateKey == "" {
		return nil, fmt.Errorf("private_key cannot be empty")
	}

	// Create GitHub App auth handler
	logger := logging.LoggerFromCtx(context.Background()).WithField("component", "github_app_auth")
	githubApp, err := NewGitHubAppAuthWithBaseURL(appID, privateKey, baseURL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App authentication: %w", err)
	}

	// Create a basic client for now (will be replaced with installation-specific clients during collection)
	client := github.NewClient(nil)

	// Set custom base URL for GitHub Enterprise
	if baseURL != "" {
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		client, err = client.WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to set GitHub Enterprise URL: %w", err)
		}
	}

	// Check for scan_all_repos option
	scanAllRepos := false
	if scanAllRaw, ok := cfg.Config["scan_all_repos"].(bool); ok {
		scanAllRepos = scanAllRaw
	}

	// Check for scan_all_organizations option
	scanAllOrganizations := false
	if scanAllOrgsRaw, ok := cfg.Config["scan_all_organizations"].(bool); ok {
		scanAllOrganizations = scanAllOrgsRaw
	}

	// Extract repository list (optional if scan_all_repos is true)
	var repositories []string
	if reposRaw, ok := cfg.Config["repositories"].([]interface{}); ok {
		for i, repo := range reposRaw {
			repoStr, ok := repo.(string)
			if !ok {
				return nil, fmt.Errorf("repository at index %d is not a string", i)
			}
			if !strings.Contains(repoStr, "/") {
				return nil, fmt.Errorf("repository '%s' must be in format 'owner/repo'", repoStr)
			}
			repositories = append(repositories, repoStr)
		}
	}

	// Validate that at least one scanning method is specified
	if !scanAllRepos && !scanAllOrganizations && len(repositories) == 0 {
		return nil, fmt.Errorf("one of 'repositories' list, 'scan_all_repos: true', or 'scan_all_organizations: true' must be specified")
	}

	// Extract organization filter for scan_all_repos mode
	var organizationFilter []string
	if orgFilterRaw, ok := cfg.Config["organization_filter"].([]interface{}); ok {
		for i, org := range orgFilterRaw {
			orgStr, ok := org.(string)
			if !ok {
				return nil, fmt.Errorf("organization_filter at index %d is not a string", i)
			}
			organizationFilter = append(organizationFilter, orgStr)
		}
	}

	// Extract AI indicators (optional, defaults to CLAUDE.md variations)
	var aiIndicators []string
	if indicatorsRaw, ok := cfg.Config["ai_indicators"].([]interface{}); ok {
		for i, indicator := range indicatorsRaw {
			indicatorStr, ok := indicator.(string)
			if !ok {
				return nil, fmt.Errorf("ai_indicator at index %d is not a string", i)
			}
			aiIndicators = append(aiIndicators, indicatorStr)
		}
	} else {
		// Default AI indicators
		aiIndicators = []string{"CLAUDE.md", "claude.md", "Claude.md", "AGENTS.md", "agents.md", ".github/copilot-instructions.md"}
	}

	// Create user mapper
	userMapper, err := mapper.NewUserIDMapper(cfg.Mapper)
	if err != nil {
		return nil, fmt.Errorf("failed to create user mapper: %w", err)
	}

	return &GitHubCollector{
		client:               client,
		repositories:         repositories,
		aiIndicators:         aiIndicators,
		scanAllRepos:         scanAllRepos,
		scanAllOrganizations: scanAllOrganizations,
		organizationFilter:   organizationFilter,
		mapper:               userMapper,
		filter:               userList,
		githubApp:            githubApp,
	}, nil
}

// Name returns the collector's name
func (g *GitHubCollector) Name() string {
	return "github"
}

// Collect scans configured repositories for AI readiness indicators and returns metrics
func (g *GitHubCollector) Collect(ctx context.Context, start, end time.Time) (map[ToolUsage]Metric, error) {
	ctx = logging.WithLoggingFields(ctx, logrus.Fields{
		"component":              "github_collector",
		"collector":              "github",
		"repositories":           len(g.repositories),
		"ai_indicators":          len(g.aiIndicators),
		"scan_all_repos":         g.scanAllRepos,
		"scan_all_organizations": g.scanAllOrganizations,
	})
	logger := logging.LoggerFromCtx(ctx)

	logger.Info("starting github ai readiness collection")

	// Initialize GitHub App installations
	logger.Info("using GitHub App authentication")
	installations, err := g.githubApp.ListInstallations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list GitHub App installations: %w", err)
	}
	g.installations = installations

	logger.WithField("installations_count", len(installations)).Info("GitHub App installations loaded")
	for _, installation := range installations {
		logger.WithFields(logrus.Fields{
			"installation_id": installation.ID,
			"account":         installation.Account,
			"repositories":    len(installation.Repositories),
		}).Debug("installation loaded")
	}

	scanTime := time.Now()
	metrics := make(map[ToolUsage]Metric)
	orgSummaries := make(map[string]*OrganizationSummaryMetrics)

	// Get list of repositories to scan
	var repositoriesToScan []string

	if g.scanAllOrganizations {
		orgRepos, err := g.getAllOrganizationRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get all organization repositories: %w", err)
		}
		repositoriesToScan = orgRepos
		logger.WithField("discovered_org_repos", len(orgRepos)).Info("discovered all organization repositories")
	} else if g.scanAllRepos {
		allRepos, err := g.getAllAccessibleRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get all accessible repositories: %w", err)
		}
		repositoriesToScan = allRepos
		logger.WithField("discovered_repos", len(allRepos)).Info("discovered all accessible repositories")
	} else {
		repositoriesToScan = g.repositories
	}

	// Combine with explicitly configured repositories (avoid duplicates)
	if (g.scanAllRepos || g.scanAllOrganizations) && len(g.repositories) > 0 {
		repoSet := make(map[string]struct{})
		for _, repo := range repositoriesToScan {
			repoSet[repo] = struct{}{}
		}
		for _, repo := range g.repositories {
			repoSet[repo] = struct{}{}
		}
		repositoriesToScan = make([]string, 0, len(repoSet))
		for repo := range repoSet {
			repositoriesToScan = append(repositoriesToScan, repo)
		}
	}

	logger.WithField("total_repos_to_scan", len(repositoriesToScan)).Info("processing repositories")

	// Process each repository
	for _, repoName := range repositoriesToScan {
		parts := strings.SplitN(repoName, "/", 2)
		if len(parts) != 2 {
			logger.WithField("repository", repoName).Error("invalid repository format, skipping")
			continue
		}

		owner, repo := parts[0], parts[1]

		logger.WithFields(logrus.Fields{
			"owner": owner,
			"repo":  repo,
		}).Debug("scanning repository")

		// Check AI readiness
		isReady, indicators, repoInfo, err := g.scanRepository(ctx, owner, repo)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"owner": owner,
				"repo":  repo,
				"error": err,
			}).Error("failed to scan repository, skipping")
			continue
		}

		// Create repository metrics
		repoMetrics := RepositoryMetrics{
			Repository:        repoName,
			Organization:      owner,
			IsAIReady:         isReady,
			AIIndicatorsFound: indicators,
			ScanTimestamp:     scanTime,
		}

		// Add repository metadata
		if repoInfo != nil {
			repoMetrics.RepositoryMetadata.Private = repoInfo.GetPrivate()
			repoMetrics.RepositoryMetadata.Language = repoInfo.Language
			repoMetrics.RepositoryMetadata.StarCount = repoInfo.GetStargazersCount()
			repoMetrics.RepositoryMetadata.ForkCount = repoInfo.GetForksCount()
			if repoInfo.PushedAt != nil {
				repoMetrics.RepositoryMetadata.LastPush = repoInfo.PushedAt.Time
			}
		}

		// Map user ID (repository name)
		mappedUserID, err := g.mapper.Map(ctx, repoName)
		if err != nil {
			logger.WithField("repository", repoName).WithError(err).Error("failed to map user ID")
			mappedUserID = repoName // fallback to original
		}

		// Check if user should be included
		if g.filter != nil && !g.filter.Include(mappedUserID) {
			logger.WithField("repository", repoName).Debug("repository not in user filter, skipping")
			continue
		}

		// Add to metrics
		toolUsage := ToolUsage{
			UserID:   mappedUserID,
			ToolName: "ai-readiness",
		}

		metrics[toolUsage] = Metric{
			UserID:   mappedUserID,
			ToolName: "ai-readiness",
			Metrics: map[string]any{
				"repository":          repoMetrics.Repository,
				"organization":        repoMetrics.Organization,
				"is_ai_ready":         repoMetrics.IsAIReady,
				"ai_indicators_found": repoMetrics.AIIndicatorsFound,
				"scan_timestamp":      repoMetrics.ScanTimestamp,
				"repository_metadata": repoMetrics.RepositoryMetadata,
			},
		}

		// Update organization summary
		if orgSummary, exists := orgSummaries[owner]; exists {
			orgSummary.TotalReposScanned++
			if isReady {
				orgSummary.AIReadyRepos++
			}
		} else {
			orgSummaries[owner] = &OrganizationSummaryMetrics{
				Organization:      owner,
				TotalReposScanned: 1,
				AIReadyRepos:      0,
				ScanTimestamp:     scanTime,
			}
			if isReady {
				orgSummaries[owner].AIReadyRepos = 1
			}
		}
	}

	// Add organization summary metrics
	for org, summary := range orgSummaries {
		if summary.TotalReposScanned > 0 {
			summary.AIReadinessPercent = (float64(summary.AIReadyRepos) / float64(summary.TotalReposScanned)) * 100
		}

		// Map organization name
		mappedOrgID, err := g.mapper.Map(ctx, org)
		if err != nil {
			logger.WithField("organization", org).WithError(err).Error("failed to map organization ID")
			mappedOrgID = org // fallback to original
		}

		// Check if organization should be included
		if g.filter != nil && !g.filter.Include(mappedOrgID) {
			logger.WithField("organization", org).Debug("organization not in user filter, skipping summary")
			continue
		}

		toolUsage := ToolUsage{
			UserID:   mappedOrgID,
			ToolName: "ai-readiness-summary",
		}

		metrics[toolUsage] = Metric{
			UserID:   mappedOrgID,
			ToolName: "ai-readiness-summary",
			Metrics: map[string]any{
				"organization":            summary.Organization,
				"total_repos_scanned":     summary.TotalReposScanned,
				"ai_ready_repos":          summary.AIReadyRepos,
				"ai_readiness_percentage": summary.AIReadinessPercent,
				"scan_timestamp":          summary.ScanTimestamp,
			},
		}
	}

	logger.WithFields(logrus.Fields{
		"total_metrics": len(metrics),
		"organizations": len(orgSummaries),
		"repositories":  len(g.repositories),
	}).Info("github ai readiness collection completed")

	return metrics, nil
}

// getAllOrganizationRepositories fetches all repositories from all organizations the authenticated user has access to
func (g *GitHubCollector) getAllOrganizationRepositories(ctx context.Context) ([]string, error) {
	logger := logging.LoggerFromCtx(ctx)

	// Use GitHub App installations to get repositories
	var allRepos []string
	for _, installation := range g.installations {
		// Apply organization filter if specified
		if len(g.organizationFilter) > 0 && !g.isOrganizationAllowed(installation.Account) {
			logger.WithField("organization", installation.Account).Debug("skipping organization - not in filter")
			continue
		}

		logger.WithFields(logrus.Fields{
			"installation_id": installation.ID,
			"account":         installation.Account,
			"repositories":    len(installation.Repositories),
		}).Debug("adding installation repositories")

		allRepos = append(allRepos, installation.Repositories...)
	}

	logger.WithField("total_org_repos", len(allRepos)).Debug("completed GitHub App organization repository discovery")
	return allRepos, nil
}

// getAllAccessibleRepositories fetches all repositories accessible through GitHub App installations
func (g *GitHubCollector) getAllAccessibleRepositories(ctx context.Context) ([]string, error) {
	logger := logging.LoggerFromCtx(ctx)

	// GitHub App authentication uses installation-based repository discovery
	var allRepos []string
	for _, installation := range g.installations {
		// Apply organization filter if specified
		if len(g.organizationFilter) > 0 && !g.isOrganizationAllowed(installation.Account) {
			logger.WithField("organization", installation.Account).Debug("skipping installation - not in filter")
			continue
		}

		logger.WithFields(logrus.Fields{
			"installation_id": installation.ID,
			"account":         installation.Account,
			"repositories":    len(installation.Repositories),
		}).Debug("adding installation repositories")

		allRepos = append(allRepos, installation.Repositories...)
	}

	logger.WithField("total_accessible_repos", len(allRepos)).Debug("completed GitHub App repository discovery")
	return allRepos, nil
}

// isOrganizationAllowed checks if an organization is in the allowed filter
func (g *GitHubCollector) isOrganizationAllowed(org string) bool {
	if len(g.organizationFilter) == 0 {
		return true // No filter means all organizations are allowed
	}

	for _, allowedOrg := range g.organizationFilter {
		if allowedOrg == org {
			return true
		}
	}
	return false
}

// getClientForRepository returns the appropriate GitHub client for the given repository owner
func (g *GitHubCollector) getClientForRepository(ctx context.Context, owner string) *github.Client {
	// Find the installation for this owner
	for _, installation := range g.installations {
		if installation.Account == owner {
			// Refresh token if needed
			if err := g.githubApp.RefreshInstallationToken(ctx, installation); err != nil {
				// Log the error but continue with potentially expired token
				logging.LoggerFromCtx(ctx).WithFields(logrus.Fields{
					"installation_id": installation.ID,
					"account":         installation.Account,
					"error":           err,
				}).Warn("failed to refresh installation token")
			}

			// Create client with installation access token
			return g.githubApp.CreateInstallationClient(ctx, installation)
		}
	}

	// Fallback to default client if no installation found
	logging.LoggerFromCtx(ctx).WithField("owner", owner).Warn("no GitHub App installation found for owner, using default client")
	return g.client
}

// scanRepository checks a single repository for AI readiness indicators
func (g *GitHubCollector) scanRepository(ctx context.Context, owner, repo string) (bool, []string, *github.Repository, error) {
	logger := logging.LoggerFromCtx(ctx)

	// Get the appropriate client for this repository
	client := g.getClientForRepository(ctx, owner)

	// Get repository information
	repoInfo, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	var foundIndicators []string

	// Check for AI indicators in the repository root
	for _, indicator := range g.aiIndicators {
		exists, err := g.checkFileExists(ctx, client, owner, repo, indicator)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"owner":     owner,
				"repo":      repo,
				"indicator": indicator,
				"error":     err,
			}).Warn("failed to check file existence")
			continue
		}

		if exists {
			foundIndicators = append(foundIndicators, indicator)
		}
	}

	isReady := len(foundIndicators) > 0

	logger.WithFields(logrus.Fields{
		"owner":            owner,
		"repo":             repo,
		"is_ai_ready":      isReady,
		"indicators_found": len(foundIndicators),
	}).Debug("repository scan completed")

	return isReady, foundIndicators, repoInfo, nil
}

// checkFileExists checks if a file exists in the repository root using GitHub Contents API
func (g *GitHubCollector) checkFileExists(ctx context.Context, client *github.Client, owner, repo, filename string) (bool, error) {
	_, _, _, err := client.Repositories.GetContents(ctx, owner, repo, filename, nil)
	if err != nil {
		// GitHub API returns 404 if file doesn't exist
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isNotFoundError checks if the error is a 404 Not Found error from GitHub API
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check for GitHub's ErrorResponse with 404 status
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		return ghErr.Response.StatusCode == 404
	}

	// Fallback: check error message
	return strings.Contains(err.Error(), "404")
}
