# AI Metrics Reporter

A Go-based application for collecting, transforming, and publishing AI usage metrics from various data sources to analytics platforms like GetDX.

## Overview

The AI Metrics Reporter provides a flexible framework for:
- **Collecting** metrics from different AI services (currently supports AWS Bedrock via S3 logs)
- **Mapping** user identifiers between different formats
- **Publishing** aggregated metrics to external platforms (currently supports GetDX)

## Features

- **Modular Architecture**: Pluggable collectors, mappers, and publishers
- **Bedrock Integration**: Collect metrics from AWS Bedrock model invocation logs stored in S3
- **User ID Mapping**: Transform user identifiers using static maps, APIs, or passthrough
- **Flexible Scheduling**: Run once or on a recurring schedule
- **Infrastructure as Code**: Terraform module for AWS infrastructure setup
- **Type-Safe**: Written in Go with strong typing and interfaces

## Architecture

```
┌─────────────┐
│  Collectors │  (Bedrock S3, ...)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Metrics   │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ UserID      │
│ Mapper      │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Publishers  │  (GetDX, ...)
└─────────────┘
```

## Project Structure

```
.
├── cmd/
│   └── ai-reporter/          # Main application entry point
├── pkg/
│   ├── collector/            # Metric collectors and interfaces
│   │   ├── bedrock.go       # AWS Bedrock S3 collector
│   │   ├── collectors.go    # Collector interface and factory
│   │   ├── grafana.go       # Grafana collector
│   │   └── metric.go        # Core metric types
│   ├── publisher/            # Metric publishers
│   │   ├── getdx.go         # GetDX publisher
│   │   └── publisher.go     # Publisher interface and factory
│   ├── mapper/               # User ID mappers
│   │   └── mapper.go        # Various mapper implementations
│   ├── users/                # User filtering and management
│   │   └── filter.go
│   ├── pipeline/             # Pipeline orchestration
│   │   └── pipeline.go
│   └── types/                # Shared types and interfaces
│       └── types.go
├── terraform/                # Infrastructure as Code
│   ├── modules/
│   │   └── bedrock-logs-bucket/  # Reusable Terraform module
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── README.md
├── config.example.yaml       # Example configuration
├── CONTRIBUTING.md           # Contributing guidelines
├── go.mod
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.21 or later
- AWS credentials configured (for Bedrock collector)
- Terraform 1.0+ (for infrastructure setup)

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/adevinta/ai-engineering-metrics.git
   cd ai-engineering-metrics
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build -o bin/ai-reporter ./cmd/ai-reporter
   ```

### Infrastructure Setup

If you're using AWS Bedrock, set up the required infrastructure:

1. Navigate to the Terraform directory:
   ```bash
   cd terraform
   ```

2. Copy and configure variables:
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   # Edit terraform.tfvars with your values
   ```

3. Apply Terraform configuration:
   ```bash
   terraform init
   terraform apply
   ```

4. Enable Bedrock logging using the output instructions:
   ```bash
   terraform output bedrock_logging_configuration
   ```

See [terraform/README.md](terraform/README.md) for detailed infrastructure documentation.

### Configuration

1. Copy the example configuration:
   ```bash
   cp config.example.yaml config.yaml
   ```

2. Edit `config.yaml` with your settings:
   ```yaml
   collectors:
     - name: bedrock-s3
       type: bedrock
       enabled: true
       config:
         bucket: my-bedrock-logs-bucket
         prefix: bedrock-logs/

   publishers:
     - name: getdx
       type: getdx
       enabled: true
       config:
         api_url: https://api.getdx.com/v1/metrics
         api_key: ${GETDX_API_KEY}

   mapper:
     type: passthrough  # or: static, api
     config: {}

   schedule:
     interval: 1h
     run_once: false
   ```

### Running

Run the application with default settings:
```bash
./bin/ai-reporter -config config.yaml
```

Run once for a specific time range:
```bash
./bin/ai-reporter \
  -config config.yaml \
  -start 2024-01-01T00:00:00Z \
  -end 2024-01-02T00:00:00Z
```

## Configuration Reference

### Collectors

#### Bedrock Collector
Collects metrics from AWS Bedrock model invocation logs stored in S3.

```yaml
collectors:
  - name: bedrock-s3
    type: bedrock
    enabled: true
    config:
      bucket: string        # S3 bucket name
      prefix: string        # S3 key prefix (optional)
```

### Publishers

#### GetDX Publisher
Publishes metrics to the GetDX platform.

```yaml
publishers:
  - name: getdx
    type: getdx
    enabled: true
    config:
      api_url: string       # GetDX API endpoint
      api_key: string       # API key (supports env vars)
```

### User ID Mappers

#### Passthrough Mapper
Returns user IDs unchanged.

```yaml
mapper:
  type: passthrough
  config: {}
```

#### Static Mapper
Maps user IDs using a predefined map.

```yaml
mapper:
  type: static
  config:
    mappings:
      session-123: user@example.com
      session-456: another@example.com
```

#### API Mapper
Fetches user mappings from an external API (not yet implemented).

```yaml
mapper:
  type: api
  config:
    api_url: string
    api_key: string
```

### Schedule

```yaml
schedule:
  interval: duration      # e.g., 1h, 30m, 24h
  run_once: bool         # If true, run once and exit
```

## Development

### Adding a New Collector

1. Implement the `collector.Collector` interface:
   ```go
   type Collector interface {
       Collect(ctx context.Context, start, end time.Time) (map[ToolUsage]Metric, error)
       Name() string
   }
   ```

2. Add your collector to `pkg/collector/`

3. Register it in the `NewCollector` factory function in `pkg/collector/collectors.go`

### Adding a New Publisher

1. Implement the `publisher.Publisher` interface:
   ```go
   type Publisher interface {
       Publish(ctx context.Context, start, end time.Time, metrics map[collector.ToolUsage]collector.Metric) error
       Name() string
   }
   ```

2. Add your publisher to `pkg/publisher/`

3. Register it in the `NewPublisher` factory function in `pkg/publisher/publisher.go`

### Adding a New Mapper

1. Implement the `types.UserIDMapper` interface:
   ```go
   type UserIDMapper interface {
       Map(ctx context.Context, userID string) (string, error)
       BatchMap(ctx context.Context, userIDs []string) (map[string]string, error)
   }
   ```

2. Add your mapper to `pkg/mapper/`

3. Register it in the `NewUserIDMapper` factory function in `pkg/mapper/mapper.go`

## Testing

Run tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

## Security Considerations

- **Secrets Management**: Use environment variables or secret management services for API keys
- **IAM Permissions**: Follow the principle of least privilege for AWS roles
- **Data Privacy**: Ensure compliance with data protection regulations when collecting user metrics
- **Encryption**: S3 buckets are encrypted at rest by default in the Terraform module

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

[Add your license here]

## Support

For issues and questions:
- Open an issue on GitHub
- Contact the maintainers

## Development Commands

Common development tasks:

```bash
go build -o bin/ai-reporter ./cmd/ai-reporter    # Build the application
go test ./...                                     # Run tests
go test -cover ./...                             # Run tests with coverage
rm -rf bin/                                      # Clean build artifacts
golangci-lint run                                # Run linter (if configured)
```

## Roadmap

- [ ] Add more collectors (OpenAI, Anthropic, etc.)
- [ ] Add more publishers (custom webhooks, databases, etc.)
- [ ] Implement API-based user ID mapper
- [ ] Add metrics aggregation and filtering
- [ ] Add Prometheus exporter
- [ ] Add comprehensive test coverage
- [ ] Add CI/CD pipeline
- [ ] Add Docker support
