# AI Engineering Metrics - Claude Context

This document provides context for AI assistants (like Claude) working with the AI Engineering Metrics codebase.

## Project Purpose

This is a Go application designed to collect AI usage metrics from various data sources (starting with AWS Bedrock), filter and transform user identifiers, and publish aggregated metrics to analytics platforms like GetDX. The goal is to provide visibility into AI service usage, costs, and patterns across an organization.

## Architecture Overview

The application follows a pipeline pattern:

```
Data Sources → Collectors → Metrics → User Filtering → User ID Mapping → Publishers → External Platforms
```

### Key Components

1. **Types (`pkg/types/types.go`)**
   - `Publisher`: Interface for sending metrics to external platforms
   - `UserIDMapper`: Interface for transforming user identifiers

2. **Collectors (`pkg/collector/`)**
   - `Metric`: Core data structure representing a single AI usage event (`pkg/collector/metric.go`)
   - `ToolUsage`: Represents a user-tool combination for grouping metrics
   - `Collector`: Interface for gathering metrics from data sources
   - `BedrockCollector`: Reads AWS Bedrock model invocation logs from S3
   - `GrafanaCollector`: Collects metrics from Grafana (new addition)
   - Factory function `NewCollector` for creating collectors

3. **Publishers (`pkg/publisher/`)**
   - `GetDXPublisher`: Sends metrics to the GetDX API
   - Each publisher implements the `Publisher` interface
   - Factory function `NewPublisher` for creating publishers

4. **Mappers (`pkg/mapper/`)**
   - `PassthroughMapper`: Returns user IDs unchanged
   - `StaticMapper`: Uses a predefined map for transformations
   - `APIMapper`: Placeholder for future API-based mapping
   - Factory function `NewUserIDMapper` for creating mappers

5. **User Management (`pkg/users/`)**
   - `UsersList`: Interface for user filtering and management
   - Filtering logic to include/exclude specific users

6. **Pipeline (`pkg/pipeline/`)**
   - Orchestrates the entire data flow
   - Handles collector execution, user filtering, mapping, and publishing

7. **Main Application (`cmd/ai-reporter/`)**
   - Entry point that sets up and runs the pipeline
   - Handles scheduling and error management

## Design Principles

1. **Interface-Based**: All major components use interfaces for extensibility
2. **Modular**: Easy to add new collectors, publishers, or mappers
3. **Configuration-Driven**: Behavior controlled via YAML config
4. **Error Resilient**: Continues processing if one collector/publisher fails
5. **Cloud-Native**: Designed to run in AWS environments (EC2, ECS, Lambda)

## Common Development Tasks

### Adding a New Collector

1. Create a new file in `pkg/collector/`
2. Implement the `collector.Collector` interface:
   - `Collect(ctx, start, end) (map[ToolUsage]Metric, error)`
   - `Name() string`
3. Add initialization logic in the `NewCollector` factory function in `pkg/collector/collectors.go`
4. Update `config.example.yaml` with configuration example
5. Document in README.md

### Adding a New Publisher

1. Create a new file in `pkg/publisher/`
2. Implement the `publisher.Publisher` interface:
   - `Publish(ctx, start, end, metrics) error`
   - `Name() string`
3. Add initialization logic in the `NewPublisher` factory function in `pkg/publisher/publisher.go`
4. Update `config.example.yaml` with configuration example
5. Document in README.md

### Modifying the Metric Structure

The `Metric` type in `pkg/collector/metric.go` is the core data structure. Changes here affect:
- All collectors (must produce compatible metrics)
- All publishers (must handle the new fields)
- The user ID mapper (if changing UserID field)
- The `ToolUsage` type which is used as a key for grouping metrics

Be cautious when modifying this structure as it's a central integration point.

## Infrastructure

The `terraform/` directory contains AWS infrastructure:
- Modularized Terraform configuration with reusable components
- `modules/bedrock-logs-bucket/`: Reusable module for creating S3 buckets for Bedrock logs
- S3 bucket for Bedrock logs with proper encryption and policies
- IAM roles and policies for accessing S3 resources
- Bucket policies allowing Bedrock to write logs

After applying Terraform, Bedrock model invocation logging must be manually enabled via AWS Console or CLI.

## Current Limitations & TODOs

1. **Limited Test Coverage**: Project has some test coverage but could be expanded
2. **Limited Error Handling**: Some error cases aren't fully handled
3. **No Retries**: Publishers don't retry failed requests
4. **No Metrics Aggregation**: Metrics are published as-is without aggregation
5. **API Mapper Not Implemented**: The API-based mapper is a stub
6. **Hardcoded Pricing**: Bedrock cost calculation uses placeholder values
7. **No Observability**: No built-in metrics/logging for the application itself
8. **Grafana Collector**: The Grafana collector implementation may need completion

## Dependencies

- `github.com/aws/aws-sdk-go-v2/*`: AWS SDK for S3 access and configuration
- `gopkg.in/yaml.v3`: YAML configuration parsing
- `github.com/stretchr/testify`: Testing framework
- `golang.org/x/sync`: Synchronization primitives

**Note**: All dependencies are managed in `go.mod` and can be updated with `go get -u ./...`

## Configuration Details

The configuration file (`config.yaml`) uses YAML and supports:
- Multiple collectors (run in sequence)
- Multiple publishers (fan-out pattern)
- Single user ID mapper (applied to all metrics)
- Scheduling options (one-time or recurring)

Environment variables can be used in config values using `${VAR_NAME}` syntax (though substitution logic isn't currently implemented - this would need to be added).

## Working with AWS

The application uses AWS SDK v2 with default credential chain:
1. Environment variables
2. Shared credentials file (~/.aws/credentials)
3. IAM role (when running on EC2/ECS)

For Bedrock collector:
- Requires `s3:GetObject` and `s3:ListBucket` permissions on the logs bucket
- Bedrock logs are in JSONL format (one JSON object per line)
- Log files are organized by date in S3

## Code Style Notes

- Standard Go formatting (`gofmt`)
- Interfaces defined in `pkg/types/`
- Implementations in `internal/`
- Avoid global state
- Context passed to all I/O operations
- Error messages wrapped with context using `fmt.Errorf("...: %w", err)`

## Useful Commands

```bash
# Build
go build -o bin/ai-reporter ./cmd/ai-reporter

# Run with custom config
./bin/ai-reporter -config config.yaml

# Run for specific time range
./bin/ai-reporter -start 2024-01-01T00:00:00Z -end 2024-01-02T00:00:00Z

# Test
go test ./...

# Test with coverage
go test -cover ./...

# Get dependencies
go mod download

# Update dependencies
go get -u ./...

# Format code
go fmt ./...

# Clean build artifacts
rm -rf bin/

# Lint (requires golangci-lint)
golangci-lint run
```

## When Helping Users

- Always check the configuration file structure when debugging issues
- Verify AWS permissions if S3/Bedrock access fails
- Check that Bedrock logging is actually enabled in AWS
- Consider time zones when working with timestamps
- Remember that metrics are processed in batches through the pipeline: collection → user filtering → mapping → publishing
- The application logs to stdout - check application logs for errors
- Check the `bin/` directory for built binaries

## Future Enhancements

Potential areas for contribution:
- Add OpenAI/Anthropic/Azure OpenAI collectors
- Implement proper cost calculation with pricing tables
- Add metrics aggregation (group by user, time window, etc.)
- Add database publishers (PostgreSQL, etc.)
- Add Prometheus exporter for observability
- Implement rate limiting for API calls
- Add configuration validation
- Add health check endpoint
- Support running as a service/daemon
