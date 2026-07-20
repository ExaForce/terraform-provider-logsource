variable "exaforce_endpoint" {
  description = "ExaForce platform endpoint (e.g. https://<tenant>.us.app.exaforce.io)"
  type        = string
}

variable "exaforce_project" {
  description = "ExaForce project name"
  type        = string
  default     = "default"
}

variable "exaforce_api_token" {
  description = "ExaForce API token (X-EXF-API-TOKEN)"
  type        = string
  sensitive   = true
}

variable "aws_region" {
  description = "AWS region of the EKS cluster"
  type        = string
}

variable "cluster_name" {
  description = "EKS cluster name to onboard to ExaForce"
  type        = string
}

variable "exabot_role_arn" {
  description = "IAM role ARN for the ExaBot Kubernetes service account (from CloudScout exaforce_aws_eks_clusters data source)"
  type        = string
}

variable "exabot_sqs_url" {
  description = "SQS queue URL for ExaBot event ingestion (from CloudScout exaforce_aws_eks_clusters data source)"
  type        = string
}

variable "bucket_id" {
  description = "S3 bucket ID for ExaBot config (from CloudScout exaforce_aws_eks_clusters data source)"
  type        = string
}

variable "destination_arn" {
  description = "CloudWatch Logs destination ARN for audit log forwarding (from CloudScout exaforce_aws_eks_clusters data source)"
  type        = string
}
