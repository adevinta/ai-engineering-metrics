output "bucket_id" {
  description = "ID of the S3 bucket for Bedrock logs"
  value       = var.create_bucket ? aws_s3_bucket.bedrock_logs[0].id : null
}

output "bucket_name" {
  description = "Name of the S3 bucket for Bedrock logs"
  value       = var.create_bucket ? aws_s3_bucket.bedrock_logs[0].bucket : null
}

output "bucket_arn" {
  description = "ARN of the S3 bucket for Bedrock logs"
  value       = var.create_bucket ? aws_s3_bucket.bedrock_logs[0].arn : data.aws_s3_bucket.bedrock_logs[0].arn
}

output "bucket_region" {
  description = "Region of the S3 bucket"
  value       = var.create_bucket ? aws_s3_bucket.bedrock_logs[0].region : null
}

output "bucket_domain_name" {
  description = "Domain name of the S3 bucket"
  value       = var.create_bucket ? aws_s3_bucket.bedrock_logs[0].bucket_domain_name : null
}

output "iam_role_arn" {
  description = "ARN of the IAM role for the AI reporter application (null if not created)"
  value       = var.create_iam_role ? aws_iam_role.ai_reporter[0].arn : null
}

output "iam_role_name" {
  description = "Name of the IAM role for the AI reporter application (null if not created)"
  value       = var.create_iam_role ? aws_iam_role.ai_reporter[0].name : null
}

output "bedrock_logging_config" {
  description = "Configuration for AWS Bedrock model invocation logging"
  value = !var.create_bucket ? null:{
    bucket_name = aws_s3_bucket.bedrock_logs[0].id
    key_prefix  = "bedrock-logs/"
    region      = data.aws_region.current.name
  }
}

output "aws_cli_command" {
  description = "AWS CLI command to enable Bedrock model invocation logging"
  value       =  <<-EOT
    aws bedrock put-model-invocation-logging-configuration \
      --region ${data.aws_region.current.name} \
      --logging-config '{
        "s3Config": {
          "bucketName": "${var.create_bucket ? aws_s3_bucket.bedrock_logs[0].id : data.aws_s3_bucket.bedrock_logs[0].id}",
          "keyPrefix": "bedrock-logs/"
        },
        "textDataDeliveryEnabled": true,
        "imageDataDeliveryEnabled": false,
        "embeddingDataDeliveryEnabled": true
      }'
  EOT
}
