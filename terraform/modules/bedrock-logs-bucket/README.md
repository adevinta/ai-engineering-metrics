# Bedrock Logs Bucket Terraform Module

This Terraform module creates an AWS S3 bucket configured to receive AWS Bedrock model invocation logs, along with the necessary bucket policies and optional IAM roles.

## Features

- Creates S3 bucket with Bedrock-compatible bucket policy
- Enables versioning (configurable)
- Server-side encryption (AES256 or KMS)
- Blocks all public access
- Configurable lifecycle policies for log retention and cost optimization
- Optional IAM role for reading logs
- Comprehensive outputs including AWS CLI commands for enabling Bedrock logging

## Usage

### Basic Example

```hcl
module "bedrock_logs" {
  source = "./modules/bedrock-logs-bucket"

  bucket_name = "my-company-bedrock-logs"

  tags = {
    Environment = "production"
    Team        = "platform"
  }
}
```

### Advanced Example

```hcl
module "bedrock_logs" {
  source = "./modules/bedrock-logs-bucket"

  bucket_name        = "my-company-bedrock-logs"
  enable_versioning  = true
  log_retention_days = 90

  # Cost optimization: transition to cheaper storage classes
  transition_to_ia_days      = 30
  transition_to_glacier_days = 90

  # Use KMS encryption
  kms_key_id = aws_kms_key.bedrock_logs.id

  # Create IAM role for AI reporter application
  create_iam_role = true
  iam_role_name   = "ai-reporter-production"
  assume_role_principals = [
    "ec2.amazonaws.com",
    "ecs-tasks.amazonaws.com"
  ]

  tags = {
    Environment = "production"
    Project     = "ai-metrics"
    Team        = "platform"
  }
}
```

### Multi-Region Example

```hcl
# Production bucket in us-east-1
module "bedrock_logs_us_east" {
  source = "./modules/bedrock-logs-bucket"

  bucket_name = "my-company-bedrock-logs-us-east-1"

  tags = {
    Environment = "production"
    Region      = "us-east-1"
  }
}

# Development bucket in eu-west-1
module "bedrock_logs_eu_west" {
  source = "./modules/bedrock-logs-bucket"

  bucket_name        = "my-company-bedrock-logs-eu-west-1"
  log_retention_days = 30  # Shorter retention for dev

  tags = {
    Environment = "development"
    Region      = "eu-west-1"
  }
}
```

## Requirements

| Name | Version |
|------|---------|
| terraform | >= 1.0 |
| aws | ~> 5.0 |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| bucket_name | Name of the S3 bucket for Bedrock logs. Must be globally unique. | `string` | n/a | yes |
| force_destroy | Allow bucket to be destroyed even if it contains objects (use with caution) | `bool` | `false` | no |
| enable_versioning | Enable versioning for the S3 bucket | `bool` | `true` | no |
| log_retention_days | Number of days to retain logs (0 = indefinite retention) | `number` | `90` | no |
| noncurrent_version_expiration_days | Number of days to retain non-current versions | `number` | `30` | no |
| transition_to_ia_days | Number of days before transitioning to STANDARD_IA storage class (0 = disabled) | `number` | `0` | no |
| transition_to_glacier_days | Number of days before transitioning to GLACIER storage class (0 = disabled) | `number` | `0` | no |
| kms_key_id | KMS key ID for S3 encryption (null = use AES256) | `string` | `null` | no |
| aws_account_id | AWS account ID (null = use current account) | `string` | `null` | no |
| create_iam_role | Whether to create an IAM role for the AI reporter application | `bool` | `false` | no |
| iam_role_name | Name of the IAM role for the AI reporter application | `string` | `"ai-reporter-role"` | no |
| assume_role_principals | List of service principals that can assume the IAM role | `list(string)` | `["ec2.amazonaws.com", "ecs-tasks.amazonaws.com"]` | no |
| tags | Tags to apply to all resources | `map(string)` | `{}` | no |

## Outputs

| Name | Description |
|------|-------------|
| bucket_id | ID of the S3 bucket for Bedrock logs |
| bucket_name | Name of the S3 bucket for Bedrock logs |
| bucket_arn | ARN of the S3 bucket for Bedrock logs |
| bucket_region | Region of the S3 bucket |
| bucket_domain_name | Domain name of the S3 bucket |
| iam_role_arn | ARN of the IAM role for the AI reporter application (null if not created) |
| iam_role_name | Name of the IAM role for the AI reporter application (null if not created) |
| bedrock_logging_config | Configuration for AWS Bedrock model invocation logging |
| aws_cli_command | AWS CLI command to enable Bedrock model invocation logging |

## Enabling Bedrock Model Invocation Logging

After creating the bucket, you need to enable Bedrock model invocation logging. The module provides the AWS CLI command as an output:

```bash
terraform output -raw aws_cli_command | bash
```

Or manually via the AWS Console:

1. Go to Amazon Bedrock console
2. Navigate to Settings > Model invocation logging
3. Enable logging and configure:
   - S3 bucket: (use the bucket name from terraform output)
   - S3 prefix: `bedrock-logs/`
   - Text data delivery: Enabled
   - Image data delivery: As needed
   - Embedding data delivery: Enabled

## Bucket Policy

The module creates a bucket policy that allows the Bedrock service to:
- Write objects to the bucket (`s3:PutObject`)
- Check bucket ACLs (`s3:GetBucketAcl`)

The policy includes conditions to restrict access to:
- Your AWS account only (`aws:SourceAccount`)
- Bedrock service only in your region (`aws:SourceArn`)

## Cost Optimization

To optimize storage costs, configure lifecycle transitions:

```hcl
module "bedrock_logs" {
  source = "./modules/bedrock-logs-bucket"

  bucket_name = "my-company-bedrock-logs"

  # Transition to STANDARD_IA after 30 days
  transition_to_ia_days = 30

  # Transition to GLACIER after 90 days
  transition_to_glacier_days = 90

  # Delete logs after 365 days
  log_retention_days = 365
}
```

Storage class pricing (approximate):
- STANDARD: $0.023/GB/month
- STANDARD_IA: $0.0125/GB/month
- GLACIER: $0.004/GB/month

## Security

The module implements AWS security best practices:

- All public access is blocked
- Server-side encryption is enabled (AES256 or KMS)
- Bucket versioning is enabled by default
- Bucket policy restricts access to Bedrock service only
- IAM role follows least-privilege principle (read-only access)

## License

This module is part of the AI Metrics Reporter project.
