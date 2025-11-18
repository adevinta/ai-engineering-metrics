variable "aws_region" {
  description = "AWS region where resources will be created"
  type        = string
  default     = "eu-west-1"
}

variable "bucket_name" {
  description = "Name of the S3 bucket for Bedrock logs. Must be globally unique."
  type        = string
}

variable "force_destroy" {
  description = "Allow bucket to be destroyed even if it contains objects (use with caution)"
  type        = bool
  default     = false
}

variable "enable_versioning" {
  description = "Enable versioning for the S3 bucket"
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "Number of days to retain logs (0 = indefinite retention)"
  type        = number
  default     = 90
}

variable "noncurrent_version_expiration_days" {
  description = "Number of days to retain non-current versions"
  type        = number
  default     = 30
}

variable "transition_to_ia_days" {
  description = "Number of days before transitioning to STANDARD_IA storage class (0 = disabled)"
  type        = number
  default     = 0
}

variable "transition_to_glacier_days" {
  description = "Number of days before transitioning to GLACIER storage class (0 = disabled)"
  type        = number
  default     = 0
}

variable "kms_key_id" {
  description = "KMS key ID for S3 encryption (null = use AES256)"
  type        = string
  default     = null
}

variable "aws_account_id" {
  description = "AWS account ID (null = use current account)"
  type        = string
  default     = null
}

variable "create_iam_role" {
  description = "Whether to create an IAM role for the AI reporter application"
  type        = bool
  default     = true
}

variable "iam_role_name" {
  description = "Name of the IAM role for the AI reporter application"
  type        = string
  default     = "ai-reporter-role"
}

variable "assume_role_principals" {
  description = "List of service principals that can assume the IAM role"
  type        = list(string)
  default     = ["ec2.amazonaws.com", "ecs-tasks.amazonaws.com"]
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default = {
    Environment = "production"
    Project     = "ai-metrics"
  }
}
