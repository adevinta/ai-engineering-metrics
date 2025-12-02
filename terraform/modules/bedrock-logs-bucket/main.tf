terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Data source for current AWS account
data "aws_caller_identity" "current" {}

# Data source for current region
data "aws_region" "current" {}

# S3 bucket for Bedrock logs
resource "aws_s3_bucket" "bedrock_logs" {
  count = var.create_bucket ? 1 : 0
  bucket        = var.bucket_name
  force_destroy = var.force_destroy

  tags = merge(
    var.tags,
    {
      Name      = var.bucket_name
      Purpose   = "bedrock-model-invocation-logs"
      ManagedBy = "terraform"
    }
  )
}

data "aws_s3_bucket" "bedrock_logs" {
  count = !var.create_bucket ? 1 : 0
  bucket = var.bucket_name
}

# Enable versioning
resource "aws_s3_bucket_versioning" "bedrock_logs" {
  count = var.create_bucket ? 1 : 0
  bucket = aws_s3_bucket.bedrock_logs[0].id

  versioning_configuration {
    status = var.enable_versioning ? "Enabled" : "Disabled"
  }
}

# Enable server-side encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "bedrock_logs" {
  count = var.create_bucket ? 1 : 0
  bucket = aws_s3_bucket.bedrock_logs[0].id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = var.kms_key_id != null ? "aws:kms" : "AES256"
      kms_master_key_id = var.kms_key_id
    }
    bucket_key_enabled = var.kms_key_id != null ? true : false
  }
}


# Block public access
resource "aws_s3_bucket_public_access_block" "bedrock_logs" {
  count = var.create_bucket ? 1 : 0
  bucket = aws_s3_bucket.bedrock_logs[0].id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}


# Lifecycle policy for log retention
resource "aws_s3_bucket_lifecycle_configuration" "bedrock_logs" {
  count  = (var.create_bucket && var.log_retention_days > 0) ? 1 : 0
  bucket = aws_s3_bucket.bedrock_logs[0].id

  rule {
    id     = "expire-old-logs"
    status = "Enabled"

    expiration {
      days = var.log_retention_days
    }

    noncurrent_version_expiration {
      noncurrent_days = var.noncurrent_version_expiration_days
    }
  }

  dynamic "rule" {
    for_each = var.transition_to_ia_days > 0 ? [1] : []
    content {
      id     = "transition-to-ia"
      status = "Enabled"

      transition {
        days          = var.transition_to_ia_days
        storage_class = "STANDARD_IA"
      }
    }
  }

  dynamic "rule" {
    for_each = var.transition_to_glacier_days > 0 ? [1] : []
    content {
      id     = "transition-to-glacier"
      status = "Enabled"

      transition {
        days          = var.transition_to_glacier_days
        storage_class = "GLACIER"
      }
    }
  }
}

# S3 bucket policy to allow Bedrock to write logs
resource "aws_s3_bucket_policy" "bedrock_logs" {
  count = var.create_bucket ? 1 : 0
  bucket = aws_s3_bucket.bedrock_logs[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AWSLogDeliveryWrite"
        Effect = "Allow"
        Principal = {
          Service = "bedrock.amazonaws.com"
        }
        Action = [
          "s3:PutObject"
        ]
        Resource = "${aws_s3_bucket.bedrock_logs[0].arn}/*"
        Condition = {
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
          ArnLike = {
            "aws:SourceArn" = "arn:aws:bedrock:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:*"
          }
        }
      },
      {
        Sid    = "AWSLogDeliveryAclCheck"
        Effect = "Allow"
        Principal = {
          Service = "bedrock.amazonaws.com"
        }
        Action   = "s3:GetBucketAcl"
        Resource = aws_s3_bucket.bedrock_logs[0].arn
        Condition = {
          StringEquals = {
            "aws:SourceAccount" = data.aws_caller_identity.current.account_id
          }
        }
      }
    ]
  })
}

# IAM role for the AI reporter application (optional)
resource "aws_iam_role" "ai_reporter" {
  count = var.create_iam_role ? 1 : 0
  name  = var.iam_role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = var.assume_role_statements
  })

  tags = var.tags
}

# IAM policy for reading S3 logs
resource "aws_iam_role_policy" "ai_reporter_s3_read" {
  name  = "s3-bedrock-logs-read"
  count = var.create_iam_role ? 1 : 0
  role  = aws_iam_role.ai_reporter[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:ListBucket"
        ]
        Resource = [
          "${var.create_bucket ? aws_s3_bucket.bedrock_logs[0].arn : data.aws_s3_bucket.bedrock_logs[0].arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/BedrockModelInvocationLogs/${data.aws_region.current.name}/*"
        ]
      }
    ]
  })
}
