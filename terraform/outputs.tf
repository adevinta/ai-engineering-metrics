output "bucket_id" {
  description = "ID of the S3 bucket for Bedrock logs"
  value       = module.bedrock_logs_bucket.bucket_id
}

output "bucket_name" {
  description = "Name of the S3 bucket for Bedrock logs"
  value       = module.bedrock_logs_bucket.bucket_name
}

output "bucket_arn" {
  description = "ARN of the S3 bucket for Bedrock logs"
  value       = module.bedrock_logs_bucket.bucket_arn
}

output "bucket_region" {
  description = "Region of the S3 bucket"
  value       = module.bedrock_logs_bucket.bucket_region
}

output "bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = module.bedrock_logs_bucket.bucket_domain_name
}

output "iam_role_arn" {
  description = "ARN of the IAM role for the AI reporter application (null if not created)"
  value       = module.bedrock_logs_bucket.iam_role_arn
}

output "iam_role_name" {
  description = "Name of the IAM role for the AI reporter application (null if not created)"
  value       = module.bedrock_logs_bucket.iam_role_name
}

output "bedrock_logging_config" {
  description = "Configuration for AWS Bedrock model invocation logging"
  value       = module.bedrock_logs_bucket.bedrock_logging_config
}

output "bedrock_logging_configuration" {
  description = "Complete instructions for enabling Bedrock model invocation logging"
  value = {
    s3_bucket    = module.bedrock_logs_bucket.bucket_name
    s3_prefix    = "bedrock-logs/"
    region       = module.bedrock_logs_bucket.bucket_region
    instructions = <<-EOT
      To enable Bedrock model invocation logging:

      Console:
      1. Go to Amazon Bedrock console (https://console.aws.amazon.com/bedrock)
      2. Navigate to Settings > Model invocation logging
      3. Enable logging and configure:
         - S3 bucket: ${module.bedrock_logs_bucket.bucket_name}
         - S3 prefix: bedrock-logs/
         - Text data delivery: Enabled
         - Image data delivery: Disabled (or as needed)
         - Embedding data delivery: Enabled

      CLI:
      ${module.bedrock_logs_bucket.aws_cli_command}
    EOT
  }
}
