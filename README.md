# AI Metrics

A comprehensive Go-based system for collecting, transforming, and publishing AI usage metrics from various data sources to analytics platforms like GetDX.

## Overview

The AI Metrics system provides a flexible framework for:
- **Collecting** metrics from different AI services (supports AWS Bedrock via S3 logs, Grafana)
- **Webhook Processing** GitHub webhooks to track development events and AI interactions
- **Mapping** user identifiers between different formats
- **Publishing** aggregated metrics to external platforms (currently supports GetDX)
- **Kubernetes Deployment** ready-to-use manifests for production deployment

## Features

- **Modular Architecture**: Pluggable collectors, mappers, and publishers
- **Multiple Data Sources**:
  - **Bedrock Integration**: Collect metrics from AWS Bedrock model invocation logs stored in S3
  - **Grafana Integration**: Collect metrics from Grafana usage data
  - **GitHub Webhooks**: Process GitHub events to track AI-related development activities
- **User ID Mapping**: Transform user identifiers using static maps, regex patterns, APIs, or passthrough
- **Environment Variable Support**: Configuration files support `${env.VAR_NAME}` substitution
- **Flexible Scheduling**: Run once or on a recurring schedule via CronJob
- **Production Ready**:
  - Kubernetes manifests with health checks, resource limits, and ingress
  - Docker containerization with multi-stage builds
  - Infrastructure as Code with Terraform modules
- **Type-Safe**: Written in Go with strong typing and interfaces

## Architecture

The application follows a pipeline pattern with parallel collection, aggregation, and fan-out publishing:

```
Pipeline
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐                             │
│  │ Collector 1 │    │ Collector 2 │    ... more collectors      │
│  │ (Bedrock)   │    │ (Grafana)   │                             │
│  │             │    │             │                             │
│  │ ┌─────────┐ │    │ ┌─────────┐ │                             │
│  │ │UserID   │ │    │ │UserID   │ │    Each collector has       │
│  │ │Mapper   │ │    │ │Mapper   │ │    its own user mapping     │
│  │ └─────────┘ │    │ └─────────┘ │                             │
│  └──────┬──────┘    └──────┬──────┘                             │
│         │                  │                                    │
│         ▼                  ▼                                    │
│  ┌─────────────────────────────────────┐                        │
│  │         Metrics Aggregation         │                        │
│  │     (by UserID + ToolName key)      │                        │
│  └─────────────────┬───────────────────┘                        │
│                    │                                            │
│                    ▼                                            │
│  ┌─────────────────────────────────────┐                        │
│  │          User Filtering             │                        │
│  │      (include/exclude users)        │                        │
│  └─────────────────┬───────────────────┘                        │
│                    │                                            │
│                    ▼                                            │
│           ┌─────────────────┐                                   │
│           │ Aggregated      │                                   │
│           │ Metrics         │                                   │
│           └────────┬────────┘                                   │
│                    │                                            │
│      ┌─────────────┼─────────────┐                              │
│      │             │             │                              │
│      ▼             ▼             ▼                              │
│ ┌─────────┐  ┌─────────┐  ┌─────────┐                           │
│ │Publisher│  │Publisher│  │Publisher│  ... fan-out to all       │
│ │(GetDX)  │  │(Custom) │  │(Prom)   │      publishers           │
│ └─────────┘  └─────────┘  └─────────┘                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Features:

- **Multiple Collectors**: Each collector runs independently and can have different user ID mapping strategies
- **Per-Collector User Mapping**: User IDs are transformed at collection time, allowing different mapping logic per data source
- **Metrics Aggregation**: All collected metrics are aggregated by `(UserID, ToolName)` key before publishing
- **User Filtering**: A single user filter applies to all metrics, allowing inclusion/exclusion of specific users
- **Fan-out Publishing**: All aggregated metrics are sent to every configured publisher in parallel

## Project Structure

```
.
├── cmd/
│   ├── collector/           # Main metrics collector application entry point
│   ├── webhook/             # GitHub webhook server entry point
│   └── dx/                  # DX utility commands
├── pkg/
│   ├── collector/           # Metric collectors and interfaces
│   │   ├── bedrock.go       # AWS Bedrock S3 collector
│   │   ├── collectors.go    # Collector interface and factory
│   │   ├── grafana.go       # Grafana collector
│   │   └── metric.go        # Core metric types
│   ├── publisher/           # Metric publishers
│   │   ├── getdx.go         # GetDX publisher
│   │   └── publisher.go     # Publisher interface and factory
│   ├── mapper/              # User ID mappers
│   │   └── mapper.go        # Various mapper implementations
│   ├── users/               # User filtering and management
│   │   └── filter.go
│   ├── pipeline/            # Pipeline orchestration
│   │   └── pipeline.go
│   ├── webhook/             # GitHub webhook handling
│   │   ├── handler.go       # HTTP handler with health checks
│   │   └── config.go        # Configuration loading
│   ├── dx/                  # GetDX API client
│   ├── lcel/                # Expression language for webhooks
│   └── types/               # Shared types and interfaces
│       └── types.go
├── manifests/               # Kubernetes deployment manifests
│   ├── base/                # Base Kustomize manifests
│   │   ├── kustomization.yaml
│   │   ├── cronjob.yaml     # AI reporter scheduled job
│   │   ├── deployment.yaml  # Webhook server deployment
│   │   ├── service.yaml     # Webhook service
│   │   ├── ingress.yaml     # Webhook ingress
│   │   └── service_account.yaml
├── terraform/                # Infrastructure as Code
│   ├── modules/
│   │   └── bedrock-logs-bucket/  # Reusable Terraform module
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── README.md
├── collector.example.yaml    # Example collector configuration
├── webhook.example.yaml      # Example webhook configuration
├── CONTRIBUTING.md           # Contributing guidelines
├── go.mod
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.21 or later
- AWS credentials configured (for Bedrock collector)
- Terraform 1.0+ (for infrastructure setup)
- Kubernetes cluster (for production deployment)

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

3. Build the applications:
   ```bash
   # Build collector
   go build -o bin/collector ./cmd/collector

   # Build webhook server
   go build -o bin/webhook ./cmd/webhook

   # Or build both with Docker
   export KO_DOCKER_REPO=ghcr.io/adevinta/ai-engineering-metrics
   go run github.com/google/ko@v0.18.0 build ./cmd/webhook -t "$tag" --platform=all --base-import-paths
   go run github.com/google/ko@v0.18.0 build ./cmd/collector -t "$tag" --platform=all --base-import-paths
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

#### AI Reporter Configuration

1. Copy the example configuration:
   ```bash
   cp collector.example.yaml collector.yaml
   ```

2. Edit `collector.yaml` with your settings:
   ```yaml
   pipelines:
     - collectors:
         - name: bedrock-s3
           type: bedrock
           enabled: true
           config:
             bucket: my-bedrock-logs-bucket
             prefix: bedrock-logs/
           user_mapping:
             type: passthrough  # or: static, regex
             config: {}

       publishers:
         - name: getdx
           type: getdx
           enabled: true
           config:
             api_url: https://api.getdx.com/v1/metrics
             api_key: ${env.GETDX_API_KEY}  # Environment variable substitution

       users:
         type: static
         config:
           user_ids:
             - user@example.com
             - admin@example.com
   ```

#### Webhook Server Configuration

1. Copy the webhook example configuration:
   ```bash
   cp webhook.example.yaml webhook.yaml
   ```

2. Configure GitHub webhook processing:
   ```yaml
   github:
     webhook_secret: ${env.GITHUB_WEBHOOK_SECRET}
     if: ${headers["X-GitHub-Event"][0] == "pull_request" || headers["X-GitHub-Event"][0] == "pull_request_review"}
     pass_thru:
     - if: ${method == "POST"}
       url: https://${env.DX_INSTANCE}.getdx.net/webhooks/github
       method: ${method}
     dx:
       track_event:
       - api_key: ${env.GETDX_API_KEY}
         if: ${body.action == "opened"}
         event_name: github.pull_request.${body.action}
         user_name: ${body.sender.login}
         is_test: ${body.action == "opened"}
         event_metadata:
           repository: ${body.repository.name}
           pull_request: ${body.pull_request.number}
           pull_request_url: ${body.pull_request.html_url}
           pull_request_title: ${body.pull_request.title}
   ```

### Running

#### AI Reporter (Metrics Collector)

Run the collector with default settings:
```bash
./bin/collector -config collector.yaml
```

Run once for a specific time range:
```bash
./bin/collector \
  -config collector.yaml \
  -start 2024-01-01T00:00:00Z \
  -end 2024-01-02T00:00:00Z
```

#### Webhook Server

Run the webhook server:
```bash
./bin/webhook -config webhook.yaml -port 8080
```

The server will:
- Process GitHub webhooks and forward events to GetDX
- Provide health check endpoints at `/health` and `/ready`
- Support environment variable substitution in configuration

### Kubernetes Deployment

Deploy to Kubernetes using the provided manifests:

```bash
# Apply base manifests
kubectl apply -k manifests/base/
```

This deploys:
- **CronJob**: AI reporter runs daily at 4 AM UTC
- **Deployment**: Webhook server with 2 replicas, health checks, and resource limits
- **Service**: ClusterIP service for the webhook server
- **Ingress**: NGINX ingress with SSL termination
- **ServiceAccount**: With optional IRSA annotations for AWS access

#### Required ConfigMaps and Secrets

Create the required configuration:
```bash
# Create configuration ConfigMap
kubectl create configmap ai-metrics \
  --from-file=config.yaml=collector.yaml \
  --from-file=webhook.yaml=webhook.yaml

# Create secrets with environment variables
kubectl create secret generic ai-collector \
  --from-literal=GETDX_API_KEY=your-api-key \
  --from-literal=AWS_REGION=us-east-1

kubectl create secret generic ai-webhook \
  --from-literal=GETDX_API_KEY=your-api-key \
  --from-literal=GITHUB_WEBHOOK_SECRET=your-webhook-secret \
  --from-literal=NAMESPACE=your-namespace
```

## Configuration Reference

### Environment Variable Support

Both collector and webhook configurations support environment variable substitution using the `${env.VARIABLE_NAME}` syntax:

```yaml
config:
  api_key: ${env.GETDX_API_KEY}
  bucket: ${env.S3_BUCKET_NAME}
  webhook_secret: ${env.GITHUB_WEBHOOK_SECRET}
```

### AI Reporter Configuration

The collector configuration follows a pipeline-based schema where you can define multiple independent pipelines:

```yaml
pipelines:
  - # Pipeline 1
    collectors: [...]
    publishers: [...]
    users: {...}
  - # Pipeline 2 (optional)
    collectors: [...]
    publishers: [...]
    users: {...}
```

### Collectors

#### Bedrock Collector
Collects metrics from AWS Bedrock model invocation logs stored in S3.

```yaml
- name: bedrock-s3
  type: bedrock
  enabled: true
  config:
    bucket: string          # S3 bucket name
    prefix: string          # S3 key prefix (optional)
    local_path: string      # Local temp directory (optional)
  user_mapping:             # Per-collector user ID mapping
    type: passthrough       # or: static, regex
    config: {}
```

#### Grafana Collector
Collects metrics from Grafana usage data.

```yaml
- name: grafana
  type: grafana
  enabled: true
  config:
    api_url: ${env.GRAFANA_URL}
    api_key: ${env.GRAFANA_API_KEY}
  user_mapping:
    type: passthrough
    config: {}
```

### Publishers

#### GetDX Publisher
Publishes metrics to the GetDX platform.

```yaml
- name: getdx
  type: getdx
  enabled: true
  config:
    api_url: string         # GetDX API endpoint
    api_key: ${env.GETDX_API_KEY}  # API key (supports env vars)
```

### Webhook Configuration

The webhook server processes GitHub events and forwards them to GetDX or other endpoints:

```yaml
github:
  webhook_secret: ${env.GITHUB_WEBHOOK_SECRET}  # GitHub webhook secret
  if: ${headers["X-GitHub-Event"][0] == "pull_request"}  # Condition expression

  # Forward raw webhooks to external URLs
  pass_thru:
  - if: ${method == "POST"}
    url: https://api.example.com/webhooks/github
    method: ${method}

  # Process and send structured events to GetDX
  dx:
    track_event:
    - api_key: ${env.GETDX_API_KEY}
      if: ${body.action == "opened"}
      event_name: github.pull_request.${body.action}
      user_name: ${body.sender.login}
      is_test: ${body.action == "opened"}
      event_metadata:
        repository: ${body.repository.name}
        pull_request: ${body.pull_request.number}
        pull_request_url: ${body.pull_request.html_url}
        pull_request_title: ${body.pull_request.title}
```

#### Webhook Expression Language (CEL)

The webhook configuration uses CEL (Common Expression Language) for dynamic processing. CEL expressions are evaluated in two contexts:

1. **Template Expressions**: `${expression}` - Used in string values for dynamic content
2. **Boolean Expressions**: Direct expressions - Used in `if` conditions for filtering

##### Available Variables

| Variable  | Type                        | Description                   | Example                            |
|-----------|-----------------------------|-------------------------------|------------------------------------|
| `headers` | `map<string, list<string>>` | HTTP request headers          | `headers["X-GitHub-Event"][0]`     |
| `body`    | `map<string, any>`          | Parsed JSON webhook payload   | `body.action`, `body.sender.login` |
| `method`  | `string`                    | HTTP method (GET, POST, etc.) | `method == "POST"`                 |
| `url`     | `string`                    | Request URL path              | `url.startsWith("/webhook")`       |
| `env`     | `map<string, string>`       | Environment variables         | `env.GITHUB_WEBHOOK_SECRET`        |

##### Expression Types and Usage

**Boolean Expressions (for `if` conditions):**
```yaml
# Filter by GitHub event type
if: ${headers["X-GitHub-Event"][0] == "pull_request"}

# Filter by action
if: ${body.action == "opened" || body.action == "synchronize"}

# Complex conditions
if: ${method == "POST" && body.repository.private == false}

# Check if field exists
if: ${has(body.pull_request) && body.pull_request.draft == false}
```

**Template Expressions (for dynamic values):**
```yaml
# Extract values from payload
event_name: github.pull_request.${body.action}
user_name: ${body.sender.login}
url: https://${env.DX_INSTANCE}.getdx.net/webhooks/github

# Complex expressions
repository_info: ${body.repository.name}@${body.repository.owner.login}
```

##### Common Expression Patterns

**GitHub Event Filtering:**
```yaml
# Pull request events only
if: ${headers["X-GitHub-Event"][0] == "pull_request"}

# Multiple event types
if: ${headers["X-GitHub-Event"][0] in ["pull_request", "pull_request_review", "issue"]}

# Specific actions
if: ${body.action in ["opened", "closed", "synchronize"]}
```

**Repository and User Filtering:**
```yaml
# Filter by repository
if: ${body.repository.name == "my-repo"}

# Filter by organization
if: ${body.repository.owner.login == "my-org"}

# Exclude bots
if: ${!body.sender.login.endsWith("[bot]")}

# Filter by user type
if: ${body.sender.type == "User"}
```

**Pull Request Specific:**
```yaml
# Only non-draft PRs
if: ${has(body.pull_request) && body.pull_request.draft == false}

# PRs from forks
if: ${body.pull_request.head.repo.fork == true}

# PRs to main branch
if: ${body.pull_request.base.ref == "main"}

# Size-based filtering
if: ${body.pull_request.additions + body.pull_request.deletions < 1000}
```

**Label and Path Filtering:**
```yaml
# Check for specific labels
if: ${body.pull_request.labels.exists(label, label.name == "needs-review")}

# Filter by file changes (if available in payload)
if: ${body.pull_request.changed_files.exists(file, file.filename.startsWith("src/"))}
```

##### Expression Evaluation Context

**Evaluation Order:**
1. **Top-level `if`**: Evaluated first to determine if webhook should be processed
2. **Pass-through `if`**: Evaluated for each pass-through target
3. **Track event `if`**: Evaluated for each GetDX event

**Error Handling:**
- Invalid expressions cause the webhook to return HTTP 500
- Missing fields in expressions evaluate to `null`
- Use `has()` function to check field existence before accessing

**Type Coercion:**
```yaml
# String comparisons
body.action == "opened"  # String equality

# Numeric comparisons
body.pull_request.number > 100  # Numeric comparison

# Boolean evaluation
body.repository.private  # Direct boolean check
body.repository.private == true  # Explicit boolean comparison
```

##### Advanced Examples

**Multi-condition event processing:**
```yaml
github:
  # Process PR events and reviews
  if: |
    ${
      headers["X-GitHub-Event"][0] == "pull_request" &&
      body.action in ["opened", "synchronize", "closed"] &&
      !body.pull_request.draft
    }

  dx:
    track_event:
    - # Track PR opened events
      if: ${body.action == "opened"}
      event_name: pr.created
      user_name: ${body.sender.login}
      event_metadata:
        pr_id: ${body.number}
        repository: ${body.repository.full_name}

    - # Track PR closed/merged events
      if: body.action == "closed"
      event_name: ${body.pull_request.merged ? "pr.merged" : "pr.closed"}
      user_name: ${body.sender.login}
      event_metadata:
        pr_id: ${body.number}
        merged: ${body.pull_request.merged}
```

**Environment-based configuration:**
```yaml
github:
  # Different behavior per environment
  if: ${method == "POST"}

  pass_thru:
  - # Forward to environment-specific endpoint
    url: https://${env.ENVIRONMENT == "prod" ? "api" : "staging-api"}.example.com/webhook
    method: ${method}

  dx:
    track_event:
    - # Mark events as test in non-prod
      is_test: ${env.ENVIRONMENT != "prod"}
      event_name: github.${headers["X-GitHub-Event"][0]}.${body.action}
```

##### Expression Testing and Debugging

To test expressions locally:
```bash
# Enable debug logging
export LOG_LEVEL=debug

# Send test webhook
curl -X POST http://localhost:8080/webhook \
  -H "X-GitHub-Event: pull_request" \
  -H "Content-Type: application/json" \
  -d @test-payload.json
```

The webhook server logs will show:
- Expression evaluation results
- Filtered events (when `if` conditions are false)
- Any expression errors

### User ID Mappers

User ID mapping is configured per-collector, allowing different mapping strategies for different data sources.

#### Passthrough Mapper
Returns user IDs unchanged.

```yaml
user_mapping:
  type: passthrough
  config: {}
```

#### Static Mapper
Maps user IDs using a predefined map.

```yaml
user_mapping:
  type: static
  config:
    session-123: user@example.com
    session-456: another@example.com
```

#### Regex Mapper
Transforms user IDs using regular expressions.

```yaml
user_mapping:
  type: regex
  config:
    regex: 'session-(.+)'
    replacement: 'user-$1@example.com'
```

### User Filtering

Controls which users are included in the metrics collection:

```yaml
users:
  type: static
  config:
    user_ids:
      - user@example.com
      - admin@example.com
      - service-account-123
```

### Time Range

Time ranges are specified via command-line flags rather than configuration:
- `-start`: Start time in RFC3339 format (default: 24 hours ago)
- `-end`: End time in RFC3339 format (default: now)

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

1. Implement the `mapper.UserIDMapper` interface:
   ```go
   type UserIDMapper interface {
       Map(ctx context.Context, userID string) (string, error)
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
# Build applications
go build -o bin/collector ./cmd/collector      # Build metrics collector
go build -o bin/webhook ./cmd/webhook # Build webhook server
go build -o bin/ ./cmd/...                          # Build all applications

# Run applications locally
./bin/collector -config collector.yaml           # Run collector once
./bin/webhook -config webhook.yaml -port 8080 # Run webhook server

# Testing
go test ./...                                       # Run tests
go test -cover ./...                               # Run tests with coverage
go test ./pkg/webhook/...                          # Test specific package

# Docker
docker build -t ai-metrics .                       # Build Docker image
docker run ai-metrics /collector -help           # Run collector in container

# Kubernetes
kubectl apply -k manifests/base/                   # Deploy to cluster
kubectl logs -f cronjob/ai-collector               # View collector logs
kubectl logs -f deployment/ai-webhook              # View webhook logs

# Maintenance
rm -rf bin/                                        # Clean build artifacts
go mod tidy                                        # Clean up dependencies
golangci-lint run                                  # Run linter (if configured)
```

## Roadmap

- [ ] Add more collectors (OpenAI, Anthropic, Azure OpenAI, etc.)
- [ ] Add more publishers (custom webhooks, databases, Prometheus, etc.)
- [ ] Implement API-based user ID mapper
- [ ] Add metrics aggregation and filtering
- [ ] Enhance webhook server with more event types
- [ ] Add comprehensive test coverage
- [ ] Add observability and monitoring
- [ ] Add rate limiting and circuit breakers
- [x] Docker support with multi-stage builds
- [x] Kubernetes manifests with health checks
- [x] GitHub webhook processing
- [x] Environment variable support in configuration
