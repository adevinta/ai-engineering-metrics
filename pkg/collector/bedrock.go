package collector

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/logging"
	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
)

type s3Client interface {
	s3.ListObjectsV2APIClient
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

var _ s3Client = (*s3.Client)(nil)

// BedrockCollector collects metrics from AWS Bedrock logs stored in S3
type BedrockCollector struct {
	localPath  string
	s3Client   s3Client
	bucketName string
	prefix     string
	toolName   string
	mapper     mapper.UserIDMapper
	filter     users.UsersList
}

var _ Collector = (*BedrockCollector)(nil)

// BedrockLogEntry represents a single log entry from Bedrock
type BedrockLogEntry struct {
	SchemaType      string    `json:"schemaType"`
	RequestID       string    `json:"requestId"`
	SchemaVersion   string    `json:"schemaVersion"`
	Timestamp       time.Time `json:"timestamp"`
	AccountID       string    `json:"accountId"`
	Identity        Identity  `json:"identity"`
	Region          string    `json:"region"`
	InferenceRegion string    `json:"inferenceRegion"`
	Operation       string    `json:"operation"`
	ModelID         string    `json:"modelId"`
	Input           Input     `json:"input"`
	Output          Output    `json:"output"`
	Error           *string   `json:"error,omitempty"`
}

// Identity contains the caller identity information
type Identity struct {
	ARN string `json:"arn"`
}

// Input contains the input token information
type Input struct {
	InputContentType          string `json:"inputContentType"`
	InputTokenCount           int    `json:"inputTokenCount"`
	CacheReadInputTokenCount  int    `json:"cacheReadInputTokenCount,omitempty"`
	CacheWriteInputTokenCount int    `json:"cacheWriteInputTokenCount,omitempty"`
}

// Output contains the output token information
type Output struct {
	OutputTokenCount  int    `json:"outputTokenCount"`
	OutputContentType string `json:"outputContentType"`
}

type AggregatedMetric struct {
	InputTokens           int `json:"input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	CacheReadInputTokens  int `json:"cache_read_input_tokens,omitempty"`
	CacheWriteInputTokens int `json:"cache_write_input_tokens,omitempty"`
}

// NewBedrockCollector creates a new Bedrock collector
func NewBedrockCollector(s3Client s3Client, localPath, bucketName, prefix, toolName string, mapper mapper.UserIDMapper, filter users.UsersList) *BedrockCollector {
	return &BedrockCollector{
		localPath:  localPath,
		s3Client:   s3Client,
		bucketName: bucketName,
		prefix:     prefix,
		toolName:   toolName,
		mapper:     mapper,
		filter:     filter,
	}
}

// Name returns the collector's name
func (c *BedrockCollector) Name() string {
	return c.toolName
}

func parseBedrockLogs(body io.Reader) ([]BedrockLogEntry, error) {
	magic := make([]byte, 2)
	_, err := body.Read(magic)
	if err != nil {
		return nil, fmt.Errorf("failed to read bedrock log bytes: %w", err)
	}
	body = io.MultiReader(bytes.NewReader(magic), body)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		body, err = gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
	}

	entries := []BedrockLogEntry{}

	decoder := json.NewDecoder(body)
	for decoder.More() {
		var entry BedrockLogEntry
		if err := decoder.Decode(&entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bedrock log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (c *BedrockCollector) aggregateFromJSONData(ctx context.Context, aggregated map[string]AggregatedMetric, from, to time.Time, reader io.Reader) error {
	entries, err := parseBedrockLogs(reader)
	if err != nil {
		return fmt.Errorf("failed to parse bedrock logs: %w", err)
	}
	for _, entry := range entries {
		if entry.Timestamp.Before(from) || entry.Timestamp.After(to) {
			continue
		}
		userID, err := c.mapper.Map(ctx, entry.Identity.ARN)
		if err != nil {
			return fmt.Errorf("failed to map user ID: %w", err)
		}
		if c.filter != nil && !c.filter.Include(userID) {
			continue
		}

		metric := aggregated[userID]
		metric.InputTokens += entry.Input.InputTokenCount
		metric.OutputTokens += entry.Output.OutputTokenCount
		metric.CacheReadInputTokens += entry.Input.CacheReadInputTokenCount
		metric.CacheWriteInputTokens += entry.Input.CacheWriteInputTokenCount
		aggregated[userID] = metric
	}
	return nil
}

func (c *BedrockCollector) aggregateFromS3(ctx context.Context, aggregated map[string]AggregatedMetric, from, to time.Time, key string) error {
	result, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get S3 object %s: %w", key, err)
	}
	defer result.Body.Close()
	return c.aggregateFromJSONData(ctx, aggregated, from, to, result.Body)
}

func (c *BedrockCollector) aggregateFromPath(ctx context.Context, aggregated map[string]AggregatedMetric, from, to time.Time, path string) error {
	body, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open path %s: %w", path, err)
	}
	defer body.Close()
	return c.aggregateFromJSONData(ctx, aggregated, from, to, body)
}

func (c *BedrockCollector) getPrefix(from, to time.Time) string {
	parts := []string{strings.TrimSuffix(c.prefix, "/")}
	if c.localPath != "" {
		parts = []string{c.localPath}
	}
	if from.Year() != to.Year() {
		return strings.Join(parts, "/") + "/"
	}
	parts = append(parts, from.Format("2006"))
	if from.Month() != to.Month() {
		return strings.Join(parts, "/") + "/"
	}
	parts = append(parts, from.Format("01"))
	if from.Day() != to.Day() {
		return strings.Join(parts, "/") + "/"
	}
	parts = append(parts, from.Format("02"))
	return strings.Join(parts, "/") + "/"
}

func (c *BedrockCollector) collectFromLocalPath(ctx context.Context, from, to time.Time, prefix string) (map[string]AggregatedMetric, error) {
	aggregated := make(map[string]AggregatedMetric)
	err := filepath.Walk(prefix, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return c.aggregateFromPath(ctx, aggregated, from, to, path)
	})
	return aggregated, err
}

func (c *BedrockCollector) collectFromS3(ctx context.Context, from, to time.Time, prefix string) (map[string]AggregatedMetric, error) {
	logger := logging.LoggerFromCtx(ctx)

	// List objects in the S3 bucket with the given prefix
	paginator := s3.NewListObjectsV2Paginator(c.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucketName),
		Prefix: aws.String(c.getPrefix(from, to)),
	})

	aggregated := make(map[string]AggregatedMetric)
	fileCount := 0

	logger.Info("listing s3 objects")

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			logger.WithError(err).Error("failed to list S3 objects")
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		logger.WithField("objects_in_page", len(page.Contents)).Debug("processing s3 page")

		for _, obj := range page.Contents {
			fileLogger := logger.WithFields(logrus.Fields{
				"s3_key": *obj.Key,
				"size": obj.Size,
				"last_modified": obj.LastModified,
			})

			fileLogger.Debug("processing s3 object")

			if err := c.aggregateFromS3(ctx, aggregated, from, to, *obj.Key); err != nil {
				fileLogger.WithError(err).Error("failed to aggregate from S3 object")
				return nil, fmt.Errorf("failed to aggregate from S3: %w", err)
			}
			fileCount++
		}
	}

	logger.WithField("files_processed", fileCount).Info("s3 collection completed")
	return aggregated, nil
}

// Collect retrieves metrics from S3 for the given time range
func (c *BedrockCollector) Collect(ctx context.Context, from, to time.Time) (map[ToolUsage]Metric, error) {
	ctx = logging.WithLoggingFields(ctx, logrus.Fields{
		"component": "bedrock_collector",
		"tool_name": c.toolName,
		"bucket": c.bucketName,
	})
	logger := logging.LoggerFromCtx(ctx)

	prefix := c.getPrefix(from, to)
	logger.WithFields(logrus.Fields{
		"from": from.Format(time.RFC3339),
		"to": to.Format(time.RFC3339),
		"prefix": prefix,
		"local_path": c.localPath,
	}).Info("starting bedrock log collection")

	collectorFunc := c.collectFromS3
	source := "s3"
	if c.localPath != "" {
		collectorFunc = c.collectFromLocalPath
		source = "local_filesystem"
	}

	logger.WithField("source", source).Info("collecting from data source")

	aggregated, err := collectorFunc(ctx, from, to, prefix)
	if err != nil {
		logger.WithError(err).Error("failed to collect metrics")
		return nil, fmt.Errorf("failed to collect metrics: %w", err)
	}

	metrics := c.renderMetrics(from, to, aggregated)
	logger.WithFields(logrus.Fields{
		"user_count": len(aggregated),
		"metric_count": len(metrics),
	}).Info("bedrock log collection completed")

	return metrics, nil
}

func (c *BedrockCollector) renderMetrics(from, to time.Time, aggregated map[string]AggregatedMetric) map[ToolUsage]Metric {
	metrics := make(map[ToolUsage]Metric)
	for userID, metric := range aggregated {
		metrics[ToolUsage{UserID: userID, ToolName: c.toolName}] = Metric{
			UserID:   userID,
			ToolName: c.toolName,
			Metrics: map[string]any{
				"input_tokens":             metric.InputTokens,
				"output_tokens":            metric.OutputTokens,
				"cache_read_input_tokens":  metric.CacheReadInputTokens,
				"cache_write_input_tokens": metric.CacheWriteInputTokens,
			},
		}
	}
	return metrics
}
