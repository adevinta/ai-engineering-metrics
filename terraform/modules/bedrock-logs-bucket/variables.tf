variable "bucket_name" {
  description = "Name of the S3 bucket for Bedrock logs. Must be globally unique."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]*[a-z0-9]$", var.bucket_name)) && length(var.bucket_name) >= 3 && length(var.bucket_name) <= 63
    error_message = "Bucket name must be between 3 and 63 characters, start and end with lowercase letter or number, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "create_bucket" {
  description = "Whether to create the S3 bucket"
  type        = bool
  default     = false
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

  validation {
    condition     = var.log_retention_days >= 0
    error_message = "Log retention days must be 0 or greater."
  }
}

variable "noncurrent_version_expiration_days" {
  description = "Number of days to retain non-current versions"
  type        = number
  default     = 30

  validation {
    condition     = var.noncurrent_version_expiration_days > 0
    error_message = "Non-current version expiration days must be greater than 0."
  }
}

variable "transition_to_ia_days" {
  description = "Number of days before transitioning to STANDARD_IA storage class (0 = disabled)"
  type        = number
  default     = 0

  validation {
    condition     = var.transition_to_ia_days >= 0
    error_message = "Transition to IA days must be 0 or greater."
  }
}

variable "transition_to_glacier_days" {
  description = "Number of days before transitioning to GLACIER storage class (0 = disabled)"
  type        = number
  default     = 0

  validation {
    condition     = var.transition_to_glacier_days >= 0
    error_message = "Transition to Glacier days must be 0 or greater."
  }
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

variable "iam_role_name" {
  description = "Name of the IAM role for the AI reporter application"
  type        = string
  default     = "ai-metrics-collector"
}

variable "assume_role_statements" {
  description = "List of statements that can assume the IAM role"
  type        = list(object({
    Effect = string
    Action = list(string)
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
  default     = {}
}
