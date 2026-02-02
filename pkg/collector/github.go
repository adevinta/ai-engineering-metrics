package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/google/go-github/v75/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
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
	// Extract GitHub token
	tokenRaw, ok := cfg.Config["github_token"].(string)
	if !ok {
		return nil, fmt.Errorf("github_token is required for github collector")
	}

	// Expand environment variables
	token, err := lcel.ExpandEnv(tokenRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to expand github_token: %w", err)
	}

	if token == "" {
		return nil, fmt.Errorf("github_token cannot be empty")
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

	// Create GitHub client with OAuth2 token
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

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

	return &GitHubCollector{
		client:               client,
		repositories:         repositories,
		aiIndicators:         aiIndicators,
		scanAllRepos:         scanAllRepos,
		scanAllOrganizations: scanAllOrganizations,
		organizationFilter:   organizationFilter,
		mapper:               userMapper,
		filter:               userList,
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

	// First, get all organizations the user has access to
	organizations, err := g.getAllAccessibleOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible organizations: %w", err)
	}

	logger.WithField("organizations_found", len(organizations)).Debug("discovered organizations")

	var allRepos []string

	// For each organization, get all repositories
	for _, orgName := range organizations {
		orgLogger := logger.WithField("organization", orgName)
		orgLogger.Debug("fetching repositories for organization")

		opts := &github.RepositoryListByOrgOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		}

		orgRepos := 0
		for {
			repos, resp, err := g.client.Repositories.ListByOrg(ctx, orgName, opts)
			if err != nil {
				orgLogger.WithError(err).Error("failed to list repositories for organization")
				break // Continue with next organization instead of failing completely
			}

			for _, repo := range repos {
				repoName := repo.GetFullName()
				allRepos = append(allRepos, repoName)
				orgRepos++
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		orgLogger.WithField("repos_found", orgRepos).Debug("completed organization repository scan")
	}

	logger.WithField("total_org_repos", len(allRepos)).Debug("completed organization repository discovery")
	return allRepos, nil
}

// getAllAccessibleOrganizations fetches all organizations the authenticated user has access to
func (g *GitHubCollector) getAllAccessibleOrganizations(ctx context.Context) ([]string, error) {
	logger := logging.LoggerFromCtx(ctx)

	var organizations []string

	opts := &github.ListOptions{PerPage: 100}

	logger.Debug("fetching accessible organizations")

	for {
		orgs, resp, err := g.client.Organizations.List(ctx, "", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list organizations: %w", err)
		}

		for _, org := range orgs {
			orgName := org.GetLogin()

			// Apply organization filter if specified
			if len(g.organizationFilter) > 0 {
				if !g.isOrganizationAllowed(orgName) {
					logger.WithField("organization", orgName).Debug("skipping organization - not in filter")
					continue
				}
			}

			organizations = append(organizations, orgName)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	logger.WithField("total_accessible_orgs", len(organizations)).Debug("completed organization discovery")
	return organizations, nil
}

// getAllAccessibleRepositories fetches all repositories the authenticated user has access to
func (g *GitHubCollector) getAllAccessibleRepositories(ctx context.Context) ([]string, error) {
	logger := logging.LoggerFromCtx(ctx)

	var allRepos []string

	// List repositories for the authenticated user
	opts := &github.RepositoryListByAuthenticatedUserOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	logger.Debug("fetching accessible repositories")

	for {
		repos, resp, err := g.client.Repositories.ListByAuthenticatedUser(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		for _, repo := range repos {
			repoName := repo.GetFullName()

			// Apply organization filter if specified
			if len(g.organizationFilter) > 0 {
				owner := repo.GetOwner().GetLogin()
				if !g.isOrganizationAllowed(owner) {
					logger.WithFields(logrus.Fields{
						"repository":   repoName,
						"organization": owner,
					}).Debug("skipping repository - organization not in filter")
					continue
				}
			}

			allRepos = append(allRepos, repoName)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	logger.WithField("total_accessible_repos", len(allRepos)).Debug("completed repository discovery")
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

// scanRepository checks a single repository for AI readiness indicators
func (g *GitHubCollector) scanRepository(ctx context.Context, owner, repo string) (bool, []string, *github.Repository, error) {
	logger := logging.LoggerFromCtx(ctx)

	// Get repository information
	repoInfo, _, err := g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return false, nil, nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	var foundIndicators []string

	// Check for AI indicators in the repository root
	for _, indicator := range g.aiIndicators {
		exists, err := g.checkFileExists(ctx, owner, repo, indicator)
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
func (g *GitHubCollector) checkFileExists(ctx context.Context, owner, repo, filename string) (bool, error) {
	_, _, _, err := g.client.Repositories.GetContents(ctx, owner, repo, filename, nil)
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
