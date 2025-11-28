package webhook

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/adevinta/ai-engineering-metrics/pkg/dx"
	"github.com/adevinta/ai-engineering-metrics/pkg/lcel"
	"github.com/goccy/go-yaml"
)

type GithubConfig struct {
	WebhookSecret string           `yaml:"webhook_secret"`
	If            string           `yaml:"if"`
	PassThru      []PassThruConfig `yaml:"pass_thru,omitempty"`
	DX            *DXConfig        `yaml:"dx,omitempty"`
}
type PassThruConfig struct {
	If     string `yaml:"if"`
	URL    string `yaml:"url"`
	Method string `yaml:"method"`
}

type DXConfig struct {
	TrackEvent []TrackEventConfig `yaml:"track_event"`
}

type TrackEventConfig struct {
	APIKey        string            `yaml:"api_key"`
	If            string            `yaml:"if"`
	IsTest        string            `yaml:"is_test"`
	EventName     string            `yaml:"event_name"`
	UserName      string            `yaml:"user_name"`
	EventMetadata map[string]string `yaml:"event_metadata"`
}

type WebhookConfig struct {
	GithubConfig GithubConfig `yaml:"github"`
}

func FromConfigFile(configFile string) HandlerOption {
	return func(h *Handler) error {
		data, err := os.ReadFile(configFile)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}
		var cfg WebhookConfig
		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			log.Fatalf("Failed to unmarshal config file: %v", err)
		}

		err = populateGitHubHandler(&h.GitHubHandler, cfg.GithubConfig)
		if err != nil {
			return fmt.Errorf("invalid github handler: %w", err)
		}

		return nil
	}
}

func populateGitHubHandler(handler *GitHubHandler, cfg GithubConfig) error {
	secret, err := lcel.ExpandEnv(cfg.WebhookSecret)
	if err != nil {
		return fmt.Errorf("invalid webhook secret expression: %w", err)
	}
	handler.webhookSecret = secret

	err = lcel.BindPtr(cfg.If, &handler.If, githubWebhookContextVariables...)
	if err != nil {
		return fmt.Errorf("invalid if expression: %w", err)
	}

	for _, passThruCfg := range cfg.PassThru {
		passThru, err := newGitHubPassThruTarget(&passThruCfg)
		if err != nil {
			return fmt.Errorf("invalid pass thru target: %w", err)
		}
		handler.PassThru = append(handler.PassThru, passThru)
	}

	github, err := newGitHubDXTarget(cfg.DX)
	if err != nil {
		return fmt.Errorf("invalid github dx target: %w", err)
	}
	handler.DX = github

	return nil
}

func newGitHubPassThruTarget(cfg *PassThruConfig) (*GitHubPassThruTarget, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.URL == "" {
		return nil, errors.New("url is required")
	}
	target := GitHubPassThruTarget{}
	if cfg.If != "" {
		if err := lcel.BindPtr(cfg.If, &target.If, githubWebhookContextVariables...); err != nil {
			return nil, fmt.Errorf("invalid if expression: %w", err)
		}
	}
	for key, err := range map[string]error{
		"url":    lcel.Bind(cfg.URL, &target.URL, githubWebhookContextVariables...),
		"method": lcel.Bind(cfg.Method, &target.Method, githubWebhookContextVariables...),
	} {
		if err != nil {
			return nil, fmt.Errorf("invalid %s expression: %w", key, err)
		}
	}
	return &target, nil
}

func newGitHubDXTarget(cfg *DXConfig) (*GitHubDXTarget, error) {
	if cfg == nil {
		return nil, nil
	}

	target := GitHubDXTarget{}

	for _, cfg := range cfg.TrackEvent {
		trackEvent, err := newGitHubTrackEventConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("invalid track event config: %w", err)
		}
		target.TrackEvent = append(target.TrackEvent, *trackEvent)
	}

	return &target, nil
}

func newGitHubTrackEventConfig(cfg TrackEventConfig) (*GitHubTrackEventTarget, error) {
	target := GitHubTrackEventTarget{}
	secret, err := lcel.ExpandEnv(cfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("invalid api key expression: %w", err)
	}

	webClient, err := dx.NewWebAPIClient(dx.WithWebAPIKey(secret))
	if err != nil {
		return nil, fmt.Errorf("invalid webhook secret expression: %w", err)
	}
	target.WebAPIClient = webClient

	if cfg.If != "" {
		if err := lcel.BindPtr(cfg.If, &target.If, githubWebhookContextVariables...); err != nil {
			return nil, fmt.Errorf("invalid if expression: %w", err)
		}
	}

	if cfg.IsTest != "" {
		if err := lcel.BindPtr(cfg.IsTest, &target.IsTest, githubWebhookContextVariables...); err != nil {
			return nil, fmt.Errorf("invalid is test expression: %w", err)
		}
	}

	userName := cfg.UserName
	if userName == "" {
		userName = "${body.sender.login}"
	}
	for key, err := range map[string]error{
		"event_name": lcel.Bind(cfg.EventName, &target.EventName, githubWebhookContextVariables...),
		"user_name":  lcel.Bind(userName, &target.UserName, githubWebhookContextVariables...),
	} {
		if err != nil {
			return nil, fmt.Errorf("invalid %s expression: %w", key, err)
		}
	}

	for k, v := range cfg.EventMetadata {
		if target.EventMetadata == nil {
			target.EventMetadata = make(map[string]lcel.ResolvedValue[any])
		}

		var metadataResolver lcel.ResolvedValue[any]

		if err := lcel.Bind(v, &metadataResolver, githubWebhookContextVariables...); err != nil {
			return nil, fmt.Errorf("invalid metadata expression for %s: %w", k, err)
		}
		target.EventMetadata[k] = metadataResolver
	}
	return &target, nil
}
