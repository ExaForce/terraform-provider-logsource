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
