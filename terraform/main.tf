terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# Create Bedrock logs bucket using the reusable module
module "bedrock_logs_bucket" {
  source = "./modules/bedrock-logs-bucket"

  bucket_name                        = var.bucket_name
  force_destroy                      = var.force_destroy
  enable_versioning                  = var.enable_versioning
  log_retention_days                 = var.log_retention_days
  noncurrent_version_expiration_days = var.noncurrent_version_expiration_days
  transition_to_ia_days              = var.transition_to_ia_days
  transition_to_glacier_days         = var.transition_to_glacier_days
  kms_key_id                         = var.kms_key_id
  aws_account_id                     = var.aws_account_id
  create_iam_role                    = var.create_iam_role
  iam_role_name                      = var.iam_role_name
  assume_role_principals             = var.assume_role_principals
  tags                               = var.tags
}
