# Contributing to AI Metrics Reporter

Thank you for your interest in contributing to the AI Metrics Reporter! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Submitting Changes](#submitting-changes)
- [Coding Standards](#coding-standards)
- [Testing](#testing)

## Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please be respectful and professional in all interactions.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/ai-metrics.git
   cd ai-metrics
   ```
3. **Add the upstream repository**:
   ```bash
   git remote add upstream https://github.com/adevinta/ai-metrics.git
   ```

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make (optional, but recommended)
- golangci-lint (for linting)
- AWS credentials (for testing Bedrock collector)
- Terraform 1.0+ (for infrastructure changes)

### Install Dependencies

```bash
make install
# or
go mod download
```

### Build the Project

```bash
make build
# or
go build -o bin/ai-reporter ./cmd/ai-reporter
```

### Run Tests

```bash
make test
# or
go test ./...
```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-openai-collector` for new features
- `fix/bedrock-timestamp-parsing` for bug fixes
- `docs/update-readme` for documentation changes
- `refactor/simplify-mapper` for refactoring

### Commit Messages

Write clear, concise commit messages:
- Use the imperative mood ("Add feature" not "Added feature")
- First line should be 50 characters or less
- Include more details in the body if necessary
- Reference issue numbers when applicable

Example:
```
Add OpenAI collector for API usage logs

- Implement OpenAI API client
- Parse usage logs into Metric format
- Add configuration options for API key
- Update documentation

Closes #123
```

## Submitting Changes

1. **Ensure your code passes all tests**:
   ```bash
   make test
   ```

2. **Format your code**:
   ```bash
   make fmt
   ```

3. **Lint your code**:
   ```bash
   make lint
   ```

4. **Commit your changes**:
   ```bash
   git add .
   git commit -m "Your descriptive commit message"
   ```

5. **Push to your fork**:
   ```bash
   git push origin your-branch-name
   ```

6. **Create a Pull Request** on GitHub:
   - Provide a clear description of the changes
   - Reference any related issues
   - Include examples or screenshots if applicable
   - Ensure all CI checks pass

## Coding Standards

### Go Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Write comments for exported functions and types
- Keep functions focused and small
- Use interfaces for abstraction
- Handle errors explicitly

### File Organization

```
internal/          # Private application code
  collector/       # Collector implementations
  publisher/       # Publisher implementations
  mapper/          # Mapper implementations
  config/          # Configuration handling

pkg/              # Public library code
  types/          # Shared types and interfaces

cmd/              # Application entry points
  ai-reporter/    # Main application
```

### Interface Design

When adding new collectors, publishers, or mappers:

1. Implement the appropriate interface from `pkg/types/`
2. Place implementation in the corresponding `internal/` directory
3. Register the new component in `cmd/ai-reporter/main.go`
4. Update `config.example.yaml` with an example configuration
5. Document in `README.md`

### Example: Adding a New Collector

```go
package collector

import (
    "context"
    "time"
    "github.com/adevinta/ai-metrics/pkg/types"
)

type MyCollector struct {
    name   string
    // ... other fields
}

func NewMyCollector(name string) *MyCollector {
    return &MyCollector{name: name}
}

func (c *MyCollector) Name() string {
    return c.name
}

func (c *MyCollector) Collect(ctx context.Context, start, end time.Time) ([]types.Metric, error) {
    // Implementation
    return nil, nil
}
```

## Testing

### Writing Tests

- Write tests for all new functionality
- Place tests in `*_test.go` files
- Use table-driven tests for multiple scenarios
- Mock external dependencies
- Aim for high test coverage

Example test structure:
```go
func TestMyCollector_Collect(t *testing.T) {
    tests := []struct {
        name    string
        start   time.Time
        end     time.Time
        want    []types.Metric
        wantErr bool
    }{
        // Test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific tests
go test -v ./internal/collector -run TestBedrockCollector
```

## Documentation

### Code Documentation

- Document all exported types, functions, and methods
- Use GoDoc format for comments
- Include examples in documentation when helpful

### User Documentation

When adding features that affect users:
- Update `README.md`
- Update `config.example.yaml`
- Update `terraform/README.md` (for infrastructure changes)
- Update `claude.md` (for architectural changes)

## Architecture Decisions

For significant architectural changes:
1. Open an issue first to discuss the approach
2. Document the decision and rationale
3. Update relevant documentation
4. Consider backward compatibility

## Getting Help

- **Questions**: Open a GitHub issue with the "question" label
- **Bugs**: Open a GitHub issue with the "bug" label
- **Feature Requests**: Open a GitHub issue with the "enhancement" label

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

Thank you for contributing! 🎉
