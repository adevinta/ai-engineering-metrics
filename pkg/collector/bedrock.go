package collector

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Client interface {
	s3.ListObjectsV2APIClient
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

var _ s3Client = (*s3.Client)(nil)

// BedrockCollector collects metrics from AWS Bedrock logs stored in S3
type BedrockCollector struct {
	s3Client   s3Client
	bucketName string
	prefix     string
	toolName   string
	mapper     mapper.Mapper
}

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
func NewBedrockCollector(s3Client s3Client, bucketName, prefix, toolName string, mapper mapper.Mapper) *BedrockCollector {
	return &BedrockCollector{
		s3Client:   s3Client,
		bucketName: bucketName,
		prefix:     prefix,
		toolName:   toolName,
		mapper:     mapper,
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
		fmt.Printf("entry: %+v\n", entry)
		if entry.Timestamp.Before(from) || entry.Timestamp.After(to) {
			continue
		}
		userID, err := c.mapper.Map(ctx, entry.Identity.ARN)
		if err != nil {
			return fmt.Errorf("failed to map user ID: %w", err)
		}

		metric := aggregated[userID]
		metric.InputTokens += entry.Input.InputTokenCount
		metric.OutputTokens += entry.Output.OutputTokenCount
		metric.CacheReadInputTokens += entry.Input.CacheReadInputTokenCount
		metric.CacheWriteInputTokens += entry.Input.CacheWriteInputTokenCount
		aggregated[userID] = metric
	}
	fmt.Printf("aggregated: %+v\n", aggregated)
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

func (c *BedrockCollector) getPrefix(from, to time.Time) string {
	parts := []string{c.prefix}
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

// Collect retrieves metrics from S3 for the given time range
func (c *BedrockCollector) Collect(ctx context.Context, from, to time.Time) (map[string]Metric, error) {

	// List objects in the S3 bucket with the given prefix
	paginator := s3.NewListObjectsV2Paginator(c.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucketName),
		Prefix: aws.String(c.getPrefix(from, to)),
	})

	aggregated := make(map[string]AggregatedMetric)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {

			if err := c.aggregateFromS3(ctx, aggregated, from, to, *obj.Key); err != nil {
				return nil, fmt.Errorf("failed to aggregate from S3: %w", err)
			}
		}
	}
	fmt.Printf("aggregated: %+v\n", aggregated)

	return c.renderMetrics(from, to, aggregated), nil
}

func (c *BedrockCollector) renderMetrics(from, to time.Time, aggregated map[string]AggregatedMetric) map[string]Metric {
	metrics := make(map[string]Metric)
	for userID, metric := range aggregated {
		metrics[userID] = Metric{
			UserID:   userID,
			From:     from,
			To:       to,
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
