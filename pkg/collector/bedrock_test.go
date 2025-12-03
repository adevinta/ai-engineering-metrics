package collector

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/adevinta/ai-engineering-metrics/pkg/mapper"
	"github.com/adevinta/ai-engineering-metrics/pkg/users"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getFileLines(t *testing.T, filename string) []string {
	t.Helper()
	var fd io.ReadCloser
	fd, err := os.Open(filename)
	require.NoError(t, err)
	defer fd.Close()
	if strings.HasSuffix(filename, ".gz") {
		gz, err := gzip.NewReader(fd)
		require.NoError(t, err)
		defer gz.Close()
		fd = gz
	}
	data, err := io.ReadAll(fd)
	require.NoError(t, err)
	return strings.Split(string(data), "\n")
}

func testFileContent(t *testing.T, filename string) {
	t.Helper()

	fd, err := os.Open(filename)
	if err != nil {
		t.Fatalf("failed to open %s: %v", filename, err)
	}
	defer fd.Close()
	entries, err := parseBedrockLogs(context.Background(), fd)
	if err != nil {
		t.Fatalf("failed to parse bedrock logs: %v", err)
	}
	t.Logf("parsed %d entries", len(entries))

	lines := getFileLines(t, filename)
	assert.Equal(t, len(entries), len(lines))

	for i, entry := range entries {
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.JSONEq(t, string(encoded), lines[i])
	}
}

func TestParseBedrockLogs(t *testing.T) {
	testFileContent(t, "test_data/metrics.json.gz")
	testFileContent(t, "test_data/metrics.json")
}

func TestGetPrefix(t *testing.T) {
	collector := NewBedrockCollector(nil, "", "test-bucket", "test-prefix", "test-tool", nil, nil)
	assert.Equal(t, "test-prefix/2021/01/01/", collector.getPrefix(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "test-prefix/2021/01/", collector.getPrefix(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "test-prefix/2021/", collector.getPrefix(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 2, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, "test-prefix/", collector.getPrefix(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)))
}

func TestBedrockCollector(t *testing.T) {
	s3Client := &s3ClientFunc{
		GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			require.Equal(t, "test-bucket", *params.Bucket)
			require.Equal(t, "test-prefix/2021/01/01/test.json.gz", *params.Key)
			fd, err := os.Open("test_data/metrics.json.gz")
			require.NoError(t, err)
			return &s3.GetObjectOutput{
				Body: fd,
			}, nil
		},
		ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			require.Equal(t, "test-bucket", *params.Bucket)
			//require.Equal(t, "test-prefix/2024/10/18/", *params.Prefix)
			return &s3.ListObjectsV2Output{
				Contents: []types.Object{
					{
						Key: aws.String("test-prefix/2021/01/01/test.json.gz"),
					},
				},
			}, nil
		},
	}
	mapper, err := mapper.NewRegexMapper(map[string]any{
		"regex":       "^arn:aws:sts::[0-9]+:assumed-role/[^/]+/(.*)",
		"replacement": "$1",
	})
	require.NoError(t, err)
	collector := NewBedrockCollector(s3Client, "", "test-bucket", "test-prefix", "test-tool", mapper, nil)
	metrics, err := collector.Collect(context.Background(), time.Date(2024, 10, 18, 0, 0, 0, 0, time.UTC), time.Date(2024, 10, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, 1, s3Client.GetObjectCalls)
	assert.Equal(t, 1, s3Client.ListObjectsV2Calls)
	assert.Equal(t, 1, len(metrics))
	assert.Contains(t, metrics, ToolUsage{UserID: "john.doe@example.com", ToolName: "test-tool"})
	assert.Equal(t, 6, metrics[ToolUsage{UserID: "john.doe@example.com", ToolName: "test-tool"}].Metrics["input_tokens"])
	assert.Equal(t, 1005, metrics[ToolUsage{UserID: "john.doe@example.com", ToolName: "test-tool"}].Metrics["output_tokens"])
	assert.Equal(t, 168574, metrics[ToolUsage{UserID: "john.doe@example.com", ToolName: "test-tool"}].Metrics["cache_read_input_tokens"])
	assert.Equal(t, 68075, metrics[ToolUsage{UserID: "john.doe@example.com", ToolName: "test-tool"}].Metrics["cache_write_input_tokens"])
}

func TestBedrockCollectorHonorsUserList(t *testing.T) {
	s3Client := &s3ClientFunc{
		GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			require.Equal(t, "test-bucket", *params.Bucket)
			require.Equal(t, "test-prefix/2021/01/01/test.json.gz", *params.Key)
			fd, err := os.Open("test_data/metrics.json.gz")
			require.NoError(t, err)
			return &s3.GetObjectOutput{
				Body: fd,
			}, nil
		},
		ListObjectsV2Func: func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			require.Equal(t, "test-bucket", *params.Bucket)
			//require.Equal(t, "test-prefix/2024/10/18/", *params.Prefix)
			return &s3.ListObjectsV2Output{
				Contents: []types.Object{
					{
						Key: aws.String("test-prefix/2021/01/01/test.json.gz"),
					},
				},
			}, nil
		},
	}
	mapper, err := mapper.NewRegexMapper(map[string]any{
		"regex":       "^arn:aws:sts::[0-9]+:assumed-role/[^/]+/(.*)",
		"replacement": "$1",
	})
	require.NoError(t, err)
	collector := NewBedrockCollector(s3Client, "", "test-bucket", "test-prefix", "test-tool", mapper, &users.StaticUserFilter{})
	metrics, err := collector.Collect(context.Background(), time.Date(2024, 10, 18, 0, 0, 0, 0, time.UTC), time.Date(2024, 10, 19, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Equal(t, 1, s3Client.GetObjectCalls)
	assert.Equal(t, 1, s3Client.ListObjectsV2Calls)
	assert.Equal(t, 0, len(metrics))
}
