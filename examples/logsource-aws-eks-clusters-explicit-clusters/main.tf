terraform {
  required_providers {
    exaforce = {
      source  = "exaforce/exaforceio"
      version = "~> 0.1"
    }
  }
}

provider "exaforce" {}

data "exaforce_aws_eks_clusters" "all" {}

output "discovered_clusters" {
  value = [for c in data.exaforce_aws_eks_clusters.all.clusters : c.name]
}

resource "exaforce_aws_logsource_eks" "clusters" {
  for_each = toset([
    "demo-eks",
    "exaforce-eks"
  ])

  spec = {
    cluster_name = each.value
  }
}
