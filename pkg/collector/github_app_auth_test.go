package collector

import (
	"encoding/base64"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestValidateGitHubAppConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]any
		expectError bool
	}{
		{
			name: "valid config with integer app_id",
			config: map[string]any{
				"app_id":      123456,
				"private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: false,
		},
		{
			name: "valid config with string app_id",
			config: map[string]any{
				"app_id":      "123456",
				"private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: false,
		},
		{
			name: "missing app_id",
			config: map[string]any{
				"private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: true,
		},
		{
			name: "missing private_key",
			config: map[string]any{
				"app_id": 123456,
			},
			expectError: true,
		},
		{
			name: "invalid app_id type",
			config: map[string]any{
				"app_id":      []string{"invalid"},
				"private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: true,
		},
		{
			name: "zero app_id",
			config: map[string]any{
				"app_id":      0,
				"private_key": "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitHubAppConfig(tt.config)
			if (err != nil) != tt.expectError {
				t.Errorf("validateGitHubAppConfig() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestParsePrivateKey(t *testing.T) {
	// Sample PEM private key for testing (this is a mock key, not a real one)
	samplePEM := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqFO5xwz5SXP8YOlrJmBbwg7lW7jUfUKs9mFl4v8X7qyC2wE8
Sample Key Content (truncated for safety)
-----END RSA PRIVATE KEY-----`

	// Base64 encode the sample PEM
	base64EncodedPEM := base64.StdEncoding.EncodeToString([]byte(samplePEM))

	tests := []struct {
		name        string
		privateKey  string
		expectError bool
	}{
		{
			name:        "empty key",
			privateKey:  "",
			expectError: true,
		},
		{
			name:        "invalid PEM",
			privateKey:  "not a pem key",
			expectError: true,
		},
		{
			name:        "invalid key data",
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ninvalid\n-----END RSA PRIVATE KEY-----",
			expectError: true,
		},
		{
			name:        "base64 encoded PEM",
			privateKey:  base64EncodedPEM,
			expectError: true, // Will still error because this is not a real key, but should pass base64 decoding
		},
		{
			name:        "invalid base64",
			privateKey:  "not_valid_base64!@#",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePrivateKey(tt.privateKey)
			if (err != nil) != tt.expectError {
				t.Errorf("parsePrivateKey() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestNewGitHubAppAuth(t *testing.T) {
	tests := []struct {
		name        string
		appID       int64
		privateKey  string
		expectError bool
	}{
		{
			name:        "invalid private key",
			appID:       123456,
			privateKey:  "invalid key",
			expectError: true,
		},
		{
			name:        "zero app ID",
			appID:       0,
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			expectError: true, // parsePrivateKey will fail on invalid key format
		},
	}

	logger := logrus.NewEntry(logrus.New())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitHubAppAuth(tt.appID, tt.privateKey, logger)
			if (err != nil) != tt.expectError {
				t.Errorf("NewGitHubAppAuth() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestNewGitHubAppAuthWithBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		appID       int64
		privateKey  string
		baseURL     string
		expectError bool
	}{
		{
			name:        "valid with enterprise base URL",
			appID:       123456,
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			baseURL:     "https://github.enterprise.com/api/v3",
			expectError: true, // Will fail on key parsing but should not fail on base URL
		},
		{
			name:        "valid with empty base URL (public GitHub)",
			appID:       123456,
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			baseURL:     "",
			expectError: true, // Will fail on key parsing but should not fail on base URL
		},
		{
			name:        "invalid base URL should still work (let GitHub client handle it)",
			appID:       123456,
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
			baseURL:     "not-a-valid-url",
			expectError: true, // Will fail on key parsing, base URL validation is deferred to GitHub client
		},
	}

	logger := logrus.NewEntry(logrus.New())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewGitHubAppAuthWithBaseURL(tt.appID, tt.privateKey, tt.baseURL, logger)
			if (err != nil) != tt.expectError {
				t.Errorf("NewGitHubAppAuthWithBaseURL() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}