terraform {
  required_providers {
    exaforce = {
      source  = "exaforce/exaforceio"
      version = "~> 0.1"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 3.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
  }
}

provider "exaforce" {
  endpoint  = var.exaforce_endpoint
  project   = "default"
  api_token = var.exaforce_api_token
}

# Region is read from AWS_DEFAULT_REGION env var or ~/.aws/config.
provider "aws" {}

# Helm provider v3: kubernetes config is a nested object (not a block).
provider "helm" {
  kubernetes = {
    host                   = data.aws_eks_cluster.this.endpoint
    cluster_ca_certificate = base64decode(data.aws_eks_cluster.this.certificate_authority[0].data)
    token                  = data.aws_eks_cluster_auth.this.token
  }
}
