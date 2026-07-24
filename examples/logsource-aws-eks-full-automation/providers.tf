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
    time = {
      source  = "hashicorp/time"
      version = "~> 0.12"
    }
  }
}

provider "exaforce" {
  endpoint  = var.exaforce_endpoint
  project   = "default"
  api_token = var.exaforce_api_token
}

provider "aws" {}

provider "helm" {}
