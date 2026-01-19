variable "aws_region" {
  description = "AWS region where resources will be created"
  type        = string
  default     = "eu-west-1"
}

variable "bucket_name" {
  description = "Name of the S3 bucket for Bedrock logs. Must be globally unique."
  type        = string
  default     = "bedrock-logs"
}

variable "create_bucket" {
  description = "Whether to create the S3 bucket"
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

variable "create_iam_role" {
  description = "Whether to create an IAM role for the AI reporter application"
  type        = bool
  default     = true
}

variable "assume_role_statements" {
  description = "List of statements that can assume the IAM role"
  type = list(object({
    Effect    = string
    Action    = list(string)
    Principal = map(list(string))
    Condition = optional(map(map(string)))
  }))

  default = [
    {
      Effect = "Allow"
      Action = ["sts:AssumeRole", "sts:TagSession"]
      Principal = {
        Service = ["ec2.amazonaws.com", "pods.eks.amazonaws.com"]
      }
      # Condition = {
      #   StringEquals = {
      #     "aws:RequestTag/kubernetes-namespace": "Namespace"
      #     "aws:RequestTag/kubernetes-service-account": "ServiceAccount"
      #   }
      # }
    }
  ]
}

variable "tags" {
  description = "Tags to apply to all resources"
  type        = map(string)
  default = {
    Environment = "production"
    Project     = "ai-metrics"
  }
}
