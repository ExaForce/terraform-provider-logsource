# terraform-provider-exaforce

Terraform provider for the [ExaForce](https://exaforce.io) security platform. Automates EKS cluster registration as log sources in ExaForce CloudScout.

## Requirements

- Terraform >= 1.0
- Go >= 1.25 (for building from source)

## Authentication

```hcl
provider "exaforce" {
  endpoint  = "https://<tenant>.us.app.exaforce.io"  # env: EXAFORCE_ENDPOINT
  project   = "default"                               # env: EXAFORCE_PROJECT
  api_token = var.exaforce_api_token                  # env: EXAFORCE_API_TOKEN
}
```

The `api_token` is sent as `X-EXF-API-TOKEN`. Obtain one from the ExaForce platform settings.

## Prerequisites

Before registering an EKS cluster, ensure:

1. **AWS connector attached** — CloudScout must have discovered the cluster
2. **EKS audit logging enabled**:
   ```hcl
   resource "aws_eks_cluster" "my_cluster" {
     name     = "my-cluster"
     role_arn = aws_iam_role.eks_cluster.arn
     enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]
   }
   ```
3. **CloudWatch subscription filter** pointing to the ExaForce destination — if missing, the provider surfaces the exact Terraform resource to add in the error output.
   ```hcl
   resource "aws_cloudwatch_log_subscription_filter" "exaforce_eks" {
     name            = "exaforce-<cluster-name>-filter-<region>"
     log_group_name  = "/aws/eks/<cluster-name>/cluster"
     filter_pattern  = "{ $.kind = \"*\" }"
     destination_arn = "arn:aws:logs:<region>:<exaforce-account-id>:destination:eks-cloudwatch-logs-destination-<project>-<region>"
   }
   ```
4. **ExaForce agent (exabot-k8s) installed on the cluster** — collects and forwards audit logs to ExaForce. The `roleArn`, `queueUrl`, and `configBucketId` values are provided in the ExaForce platform after the log source is registered.
   ```bash
   helm repo add exaforce https://exaforce.github.io/helm-charts
   helm repo update

   helm install exabot-k8s exaforce/exabot-k8s \
     --set exabotK8s.serviceAccount.roleArn="<iam-role-arn>" \
     --set exabotK8s.env.queueUrl="<sqs-queue-url>" \
     --set exabotK8s.env.configBucketId="<config-bucket-id>"
   ```

## Usage

### Option 1: All clusters with exclude

Use this for bulk onboarding — discovers all EKS clusters from CloudScout and registers all of them except the ones you explicitly exclude. The `discovered_clusters` output shows what CloudScout has found during `terraform plan`.

```hcl
provider "exaforce" {
  endpoint  = "https://<tenant>.us.app.exaforce.io"
  project   = "default"
  api_token = var.exaforce_api_token
}

data "exaforce_aws_eks_clusters" "all" {}

output "discovered_clusters" {
  value = [for c in data.exaforce_aws_eks_clusters.all.clusters : c.name]
}

locals {
  exclude = toset(["test-cluster", "old-cluster"])
  target  = setsubtract(
    toset([for c in data.exaforce_aws_eks_clusters.all.clusters : c.name]),
    local.exclude
  )
}

resource "exaforce_aws_logsource_eks" "clusters" {
  for_each = local.target

  spec = {
    cluster_name = each.value
  }
}
```

`terraform plan` output:
```
data.exaforce_aws_eks_clusters.all: Reading...
data.exaforce_aws_eks_clusters.all: Read complete after 1s

Terraform will perform the following actions:

  # exaforce_aws_logsource_eks.clusters["xxx-eks"] will be created
  + resource "exaforce_aws_logsource_eks" "clusters" {
      + id   = (known after apply)
      + name = (known after apply)
      + spec = {
          + cluster_name = "xxx-eks"
          + eks_arn      = (known after apply)
        }
    }

Plan: 1 to add, 0 to change, 0 to destroy.

Changes to Outputs:
  + discovered_clusters = [
      + "xxx-eks",
      + "yyy-eks",
      + ...
    ]
```

### Option 2: Explicit cluster names

Use this when you want to register specific clusters. The provider looks up each cluster name in CloudScout discovery and resolves the ARN automatically.

```hcl
provider "exaforce" {
  endpoint  = "https://<tenant>.us.app.exaforce.io"
  project   = "default"
  api_token = var.exaforce_api_token
}

resource "exaforce_aws_logsource_eks" "clusters" {
  for_each = toset([
    "prod-eks",
    "staging-eks",
  ])

  spec = {
    cluster_name = each.value
  }
}
```

## Resources

| Resource | Description |
|---|---|
| `exaforce_aws_logsource_eks` | Registers an EKS cluster as a log source in ExaForce CloudScout |

## Data Sources

| Data Source | Description |
|---|---|
| `exaforce_aws_eks_clusters` | Lists all EKS clusters discovered by CloudScout |
| `exaforce_aws_logsources` | Lists all registered AWS log sources |

## Building from Source

```bash
git clone https://github.com/ExaForce/terraform-provider-exaforceio
cd terraform-provider-exaforceio
make build
```

To use a local build with Terraform, add a `.terraformrc` override:

```hcl
provider_installation {
  dev_overrides {
    "exaforce/exaforceio" = "/path/to/terraform-provider-exaforceio"
  }
  direct {}
}
```
