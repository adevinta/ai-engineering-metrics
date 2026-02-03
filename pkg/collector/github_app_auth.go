package collector

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v75/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

// GitHubAppAuth handles GitHub App authentication flow
type GitHubAppAuth struct {
	appID      int64
	privateKey *rsa.PrivateKey
	client     *github.Client
	logger     *logrus.Entry
	baseURL    string // GitHub Enterprise base URL (empty for public GitHub)
}

// Installation represents a GitHub App installation
type Installation struct {
	ID           int64
	Account      string
	AccessToken  string
	ExpiresAt    time.Time
	Repositories []string // Repository names accessible to this installation
}

// NewGitHubAppAuth creates a new GitHub App authentication client
func NewGitHubAppAuth(appID int64, privateKeyPEM string, logger *logrus.Entry) (*GitHubAppAuth, error) {
	return NewGitHubAppAuthWithBaseURL(appID, privateKeyPEM, "", logger)
}

// NewGitHubAppAuthWithBaseURL creates a new GitHub App authentication client with custom base URL
func NewGitHubAppAuthWithBaseURL(appID int64, privateKeyPEM string, baseURL string, logger *logrus.Entry) (*GitHubAppAuth, error) {
	// Parse private key
	privateKey, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create initial client with no authentication (will be enhanced with JWT)
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

	return &GitHubAppAuth{
		appID:      appID,
		privateKey: privateKey,
		client:     client,
		logger:     logger,
		baseURL:    baseURL,
	}, nil
}

// parsePrivateKey parses RSA private key from PEM format or base64 encoded PEM
func parsePrivateKey(privateKeyInput string) (*rsa.PrivateKey, error) {
	privateKeyPEM := privateKeyInput

	// Check if input looks like base64 encoded data (no PEM headers and only base64 characters)
	if !strings.Contains(privateKeyInput, "-----BEGIN") && !strings.Contains(privateKeyInput, "-----END") {
		// Try to decode as base64
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyInput))
		if err != nil {
			// If base64 decoding fails, try base64 URL encoding
			decoded, err = base64.URLEncoding.DecodeString(strings.TrimSpace(privateKeyInput))
			if err != nil {
				return nil, fmt.Errorf("input appears to be base64 but failed to decode: %w", err)
			}
		}
		privateKeyPEM = string(decoded)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block (input may be invalid PEM format or incorrectly encoded)")
	}

	// Try PKCS#1 format first
	if privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return privateKey, nil
	}

	// Try PKCS#8 format
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not RSA format")
	}

	return rsaKey, nil
}

// createClientWithAuth creates a GitHub client with authentication and proper base URL
func (auth *GitHubAppAuth) createClientWithAuth(ctx context.Context, token string) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Set custom base URL for GitHub Enterprise if configured
	if auth.baseURL != "" {
		baseURL := auth.baseURL
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		var err error
		client, err = client.WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to set GitHub Enterprise URL: %w", err)
		}
	}

	return client, nil
}

// generateJWT creates a JWT token for GitHub App authentication
func (auth *GitHubAppAuth) generateJWT() (string, error) {
	now := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": auth.appID,                        // Issuer: GitHub App ID
		"iat": now.Unix(),                        // Issued at
		"exp": now.Add(10 * time.Minute).Unix(),  // Expires in 10 minutes (GitHub max)
	})

	tokenString, err := token.SignedString(auth.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return tokenString, nil
}

// ListInstallations fetches all installations for the GitHub App
func (auth *GitHubAppAuth) ListInstallations(ctx context.Context) ([]*Installation, error) {
	// Generate JWT for app authentication
	jwtToken, err := auth.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}

	// Create authenticated client with JWT
	client, err := auth.createClientWithAuth(ctx, jwtToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// List all installations
	opts := &github.ListOptions{PerPage: 100}
	var allInstallations []*github.Installation

	auth.logger.Debug("fetching GitHub App installations")

	for {
		installations, resp, err := client.Apps.ListInstallations(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list installations: %w", err)
		}

		allInstallations = append(allInstallations, installations...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	auth.logger.WithField("installations_found", len(allInstallations)).Debug("fetched GitHub App installations")

	// Convert to our Installation struct and generate access tokens
	var installations []*Installation
	for _, install := range allInstallations {
		installation := &Installation{
			ID:      install.GetID(),
			Account: install.GetAccount().GetLogin(),
		}

		// Generate access token for this installation
		accessToken, expiresAt, err := auth.generateInstallationAccessToken(ctx, install.GetID())
		if err != nil {
			auth.logger.WithFields(logrus.Fields{
				"installation_id": install.GetID(),
				"account":         installation.Account,
				"error":          err,
			}).Warn("failed to generate access token for installation, skipping")
			continue
		}

		installation.AccessToken = accessToken
		installation.ExpiresAt = expiresAt

		// Get repositories for this installation
		repositories, err := auth.getInstallationRepositories(ctx, accessToken, install.GetID())
		if err != nil {
			auth.logger.WithFields(logrus.Fields{
				"installation_id": install.GetID(),
				"account":         installation.Account,
				"error":          err,
			}).Warn("failed to get installation repositories")
			repositories = []string{} // Continue with empty repository list
		}
		installation.Repositories = repositories

		installations = append(installations, installation)
	}

	auth.logger.WithField("successful_installations", len(installations)).Info("successfully authenticated installations")

	return installations, nil
}

// generateInstallationAccessToken creates an access token for a specific installation
func (auth *GitHubAppAuth) generateInstallationAccessToken(ctx context.Context, installationID int64) (string, time.Time, error) {
	// Generate JWT for app authentication
	jwtToken, err := auth.generateJWT()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate JWT token: %w", err)
	}

	// Create authenticated client with JWT
	client, err := auth.createClientWithAuth(ctx, jwtToken)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	// Create installation access token
	token, _, err := client.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create installation token: %w", err)
	}

	return token.GetToken(), token.GetExpiresAt().Time, nil
}

// getInstallationRepositories fetches repositories accessible by an installation
func (auth *GitHubAppAuth) getInstallationRepositories(ctx context.Context, accessToken string, installationID int64) ([]string, error) {
	// Create client with installation access token
	client, err := auth.createClientWithAuth(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated client: %w", err)
	}

	opts := &github.ListOptions{PerPage: 100}
	var repositories []string

	for {
		repos, resp, err := client.Apps.ListRepos(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list installation repositories: %w", err)
		}

		for _, repo := range repos.Repositories {
			repositories = append(repositories, repo.GetFullName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	auth.logger.WithFields(logrus.Fields{
		"installation_id": installationID,
		"repositories":    len(repositories),
	}).Debug("fetched installation repositories")

	return repositories, nil
}

// CreateInstallationClient creates a GitHub client authenticated with an installation access token
func (auth *GitHubAppAuth) CreateInstallationClient(ctx context.Context, installation *Installation) *github.Client {
	client, err := auth.createClientWithAuth(ctx, installation.AccessToken)
	if err != nil {
		// Log error but return a basic client to maintain backward compatibility
		auth.logger.WithError(err).Warn("failed to create client with base URL, falling back to default")
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: installation.AccessToken})
		tc := oauth2.NewClient(ctx, ts)
		return github.NewClient(tc)
	}
	return client
}

// RefreshInstallationToken refreshes an installation access token if it's expired or about to expire
func (auth *GitHubAppAuth) RefreshInstallationToken(ctx context.Context, installation *Installation) error {
	// Check if token is expired or expires within 5 minutes
	if time.Until(installation.ExpiresAt) > 5*time.Minute {
		return nil // Token is still valid
	}

	auth.logger.WithFields(logrus.Fields{
		"installation_id": installation.ID,
		"account":         installation.Account,
		"expires_at":      installation.ExpiresAt,
	}).Debug("refreshing installation access token")

	accessToken, expiresAt, err := auth.generateInstallationAccessToken(ctx, installation.ID)
	if err != nil {
		return fmt.Errorf("failed to refresh installation token: %w", err)
	}

	installation.AccessToken = accessToken
	installation.ExpiresAt = expiresAt

	auth.logger.WithFields(logrus.Fields{
		"installation_id": installation.ID,
		"account":         installation.Account,
		"new_expires_at":  installation.ExpiresAt,
	}).Debug("successfully refreshed installation access token")

	return nil
}

// validateGitHubAppConfig validates GitHub App configuration parameters
func validateGitHubAppConfig(config map[string]any) error {
	// Check for app_id
	appIDRaw, hasAppID := config["app_id"]
	if !hasAppID {
		return fmt.Errorf("app_id is required for GitHub App authentication")
	}

	// Handle app_id as string or number
	var appIDInt int64
	switch v := appIDRaw.(type) {
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("app_id must be a valid integer: %w", err)
		}
		appIDInt = parsed
	case int:
		appIDInt = int64(v)
	case int64:
		appIDInt = v
	case float64:
		appIDInt = int64(v)
	default:
		return fmt.Errorf("app_id must be a number or string, got %T", appIDRaw)
	}

	if appIDInt <= 0 {
		return fmt.Errorf("app_id must be a positive integer")
	}

	// Check for private_key
	_, hasPrivateKey := config["private_key"]
	if !hasPrivateKey {
		return fmt.Errorf("private_key is required for GitHub App authentication")
	}

	return nil
}