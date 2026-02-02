package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type ToolUsage struct {
	UserID   string
	ToolName string
}

// Metric represents a single AI usage metric
type Metric struct {
	UserID   string         `json:"user_id"`
	ToolName string         `json:"tool_name"`
	Metrics  map[string]any `json:"metrics"`
}

// Collector defines the interface for collecting metrics from various data sources
type Collector interface {
	// Collect retrieves metrics from the data source for the given time range
	Collect(ctx context.Context, start, end time.Time) (map[ToolUsage]Metric, error)
	// Name returns the collector's name
	Name() string
}

// CollectorConfig represents configuration for a single collector
type CollectorConfig struct {
	Name    string              `yaml:"name"`
	Type    string              `yaml:"type"`
	Enabled bool                `yaml:"enabled"`
	Mapper  mapper.MapperConfig `yaml:"user_mapping"`
	Config  map[string]any      `yaml:"config"`
}

func NewCollector(cfg CollectorConfig, userList users.UsersList) (Collector, error) {
	switch cfg.Type {
	case "bedrock":
		awsCfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		chainIntf := cfg.Config["assume_role_chain"]
		if chainIntf != nil {
			chain := chainIntf.([]any)
			for _, role := range chain {
				roleMap := role.(map[string]any)
				roleArn := roleMap["arn"].(string)

				stsClient := sts.NewFromConfig(awsCfg)

				creds := stscreds.NewAssumeRoleProvider(stsClient, roleArn, func(p *stscreds.AssumeRoleOptions) {
					if externalId, ok := roleMap["external_id"].(string); ok {
						p.ExternalID = aws.String(externalId)
					}
					if sessionName, ok := roleMap["session_name"].(string); ok {
						p.RoleSessionName = sessionName
					}
				})

				awsCfg.Credentials = aws.NewCredentialsCache(creds)
			}

		}

		s3Client := s3.NewFromConfig(awsCfg)

		bucketName, ok := cfg.Config["bucket"].(string)
		if !ok {
			return nil, fmt.Errorf("bucket is not a string")
		}
		prefix, ok := cfg.Config["prefix"].(string)
		if !ok {
			return nil, fmt.Errorf("prefix is not a string")
		}

		mapper, err := mapper.NewUserIDMapper(cfg.Mapper)
		if err != nil {
			return nil, fmt.Errorf("failed to create mapper: %w", err)
		}

		localPathIntf, ok := cfg.Config["local_path"]
		localPath := ""
		if ok {
			localPath, ok = localPathIntf.(string)
			if !ok {
				return nil, fmt.Errorf("local_path is not a string")
			}
		}

		c := NewBedrockCollector(s3Client, localPath, bucketName, prefix, cfg.Name, mapper, userList)
		return c, nil
	case "github":
		return NewGitHubCollector(cfg, userList)
	default:
		return nil, fmt.Errorf("unknown collector type: %s", cfg.Type)
	}
}
