# Terraform Infrastructure for AI Metrics

This directory contains Terraform configuration for provisioning AWS infrastructure to collect AI metrics from AWS Bedrock.

## Overview

The infrastructure uses a reusable Terraform module (`modules/bedrock-logs-bucket`) to provision:

- S3 bucket for storing Bedrock model invocation logs
- Bucket policies to allow Bedrock to write logs
- Optional IAM role for the AI reporter application to read logs
- Lifecycle policies for log retention and cost optimization
- Server-side encryption (AES256 or KMS)
- Versioning and public access blocking

## Prerequisites

- Terraform >= 1.0
- AWS CLI configured with appropriate credentials
- Permissions to create S3 buckets and IAM resources

## Usage

1. Copy the example variables file:
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

2. Edit `terraform.tfvars` with your desired values:
   ```hcl
   bucket_name = "my-company-bedrock-logs"
   aws_region  = "us-east-1"
   ```

3. Initialize Terraform:
   ```bash
   terraform init
   ```

4. Review the planned changes:
   ```bash
   terraform plan
   ```

5. Apply the configuration:
   ```bash
   terraform apply
   ```

6. Configure Bedrock logging using the output instructions:
   ```bash
   terraform output bedrock_logging_configuration
   ```

## Module Structure

```
terraform/
├── main.tf                          # Root module that instantiates bedrock-logs-bucket
├── variables.tf                     # Input variables
├── outputs.tf                       # Output values
├── terraform.tfvars.example         # Example variable values
├── README.md                        # This file
└── modules/
    └── bedrock-logs-bucket/         # Reusable module for Bedrock log collection
        ├── main.tf
        ├── variables.tf
        ├── outputs.tf
        └── README.md                # Module documentation
```

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|----------|
| aws_region | AWS region where resources will be created | string | "us-east-1" | no |
| bucket_name | Name of the S3 bucket for Bedrock logs (must be globally unique) | string | n/a | yes |
| force_destroy | Allow bucket to be destroyed even if it contains objects | bool | false | no |
| enable_versioning | Enable versioning for the S3 bucket | bool | true | no |
| log_retention_days | Number of days to retain logs (0 = indefinite) | number | 90 | no |
| noncurrent_version_expiration_days | Number of days to retain non-current versions | number | 30 | no |
| transition_to_ia_days | Days before transitioning to STANDARD_IA (0 = disabled) | number | 0 | no |
| transition_to_glacier_days | Days before transitioning to GLACIER (0 = disabled) | number | 0 | no |
| kms_key_id | KMS key ID for S3 encryption (null = use AES256) | string | null | no |
| aws_account_id | AWS account ID (null = use current account) | string | null | no |
| create_iam_role | Whether to create an IAM role for the AI reporter | bool | true | no |
| iam_role_name | Name of the IAM role | string | "ai-reporter-role" | no |
| assume_role_principals | Service principals that can assume the role | list(string) | ["ec2.amazonaws.com", "ecs-tasks.amazonaws.com"] | no |
| tags | Tags to apply to all resources | map(string) | see variables.tf | no |

## Outputs

| Name | Description |
|------|-------------|
| bucket_id | ID of the S3 bucket for Bedrock logs |
| bucket_name | Name of the S3 bucket for Bedrock logs |
| bucket_arn | ARN of the S3 bucket |
| bucket_region | Region of the S3 bucket |
| bucket_domain_name | Domain name of the S3 bucket |
| iam_role_arn | ARN of the IAM role (if created) |
| iam_role_name | Name of the IAM role (if created) |
| bedrock_logging_config | Configuration object for Bedrock logging |
| bedrock_logging_configuration | Complete instructions for configuring Bedrock logging |

## Enabling Bedrock Model Invocation Logging

After applying this Terraform configuration, you need to enable model invocation logging in Bedrock. You can do this via:

### AWS Console
1. Navigate to Amazon Bedrock console
2. Go to Settings > Model invocation logging
3. Enable logging and configure the S3 bucket created by this module

### AWS CLI
```bash
aws bedrock put-model-invocation-logging-configuration \
  --logging-config '{
    "s3Config": {
      "bucketName": "<bucket-name-from-terraform-output>",
      "keyPrefix": "bedrock-logs/"
    },
    "textDataDeliveryEnabled": true,
    "embeddingDataDeliveryEnabled": true
  }'
```

## Security Considerations

- The S3 bucket has public access blocked by default
- Server-side encryption is enabled using AES256
- Bucket versioning is enabled
- IAM policies follow the principle of least privilege
- The bucket policy restricts access to the Bedrock service from your AWS account only

## Clean Up

To destroy all resources created by this module:

```bash
terraform destroy
```

**Note**: Ensure the S3 bucket is empty before destroying, or set the bucket's `force_destroy` attribute to `true`.

## Using the Reusable Module

The `bedrock-logs-bucket` module can be used independently in other Terraform projects:

```hcl
module "bedrock_logs" {
  source = "path/to/ai-metrics/terraform/modules/bedrock-logs-bucket"

  bucket_name        = "my-company-bedrock-logs"
  log_retention_days = 90

  # Cost optimization
  transition_to_ia_days      = 30
  transition_to_glacier_days = 90

  tags = {
    Environment = "production"
    Team        = "platform"
  }
}
```

See the [module documentation](modules/bedrock-logs-bucket/README.md) for detailed usage examples and configuration options.
