# Discover all EKS clusters known to CloudScout.
# Provides exabot_role_arn, exabot_sqs_url, bucket_id, and destination_arn
# for the target cluster — no manual lookup needed.
data "exaforce_aws_eks_clusters" "all" {}

locals {
  cluster = one([
    for c in data.exaforce_aws_eks_clusters.all.clusters : c
    if c.name == var.cluster_name
  ])
}

# Used by the helm provider (defined in providers.tf).
data "aws_eks_cluster" "this" {
  name = var.cluster_name
}

data "aws_eks_cluster_auth" "this" {
  name = var.cluster_name
}

# Step 1: Install the ExaForce agent on the cluster.
# Values are sourced directly from the CloudScout data source.
resource "helm_release" "exabot" {
  name             = "exabot-k8s"
  repository       = "https://exaforce.github.io/helm-charts"
  chart            = "exabot-k8s"
  namespace        = "exaforce"
  create_namespace = true

  set {
    name  = "exabotK8s.serviceAccount.roleArn"
    value = local.cluster.exabot_role_arn
  }
  set {
    name  = "exabotK8s.env.queueUrl"
    value = local.cluster.exabot_sqs_url
  }
  set {
    name  = "exabotK8s.env.configBucketId"
    value = local.cluster.bucket_id
  }
}

# Step 2: Create the CloudWatch subscription filter to forward audit logs
# to the ExaForce destination. destination_arn comes from CloudScout.
resource "aws_cloudwatch_log_subscription_filter" "exaforce" {
  name            = "exaforce-${var.cluster_name}-filter-${local.cluster.region}"
  log_group_name  = "/aws/eks/${var.cluster_name}/cluster"
  filter_pattern  = "{ $.kind = \"*\" }"
  destination_arn = local.cluster.destination_arn
}

# Step 3: Register the EKS cluster as a log source in ExaForce CloudScout.
# Depends on helm and subscription filter being in place first so the
# provider's prerequisite checks pass on first apply.
resource "exaforce_aws_logsource_eks" "this" {
  spec = {
    cluster_name = var.cluster_name
  }

  depends_on = [
    helm_release.exabot,
    aws_cloudwatch_log_subscription_filter.exaforce,
  ]
}
