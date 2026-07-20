# Used by the helm provider (defined in providers.tf) for cluster authentication.
data "aws_eks_cluster" "this" {
  name = var.cluster_name
}

data "aws_eks_cluster_auth" "this" {
  name = var.cluster_name
}

# Step 1: Install the ExaForce agent on the cluster.
# Values (role ARN, SQS URL, bucket ID) are obtained from CloudScout
# and passed in as variables — use the exaforce_aws_eks_clusters data
# source or the CloudScout UI to look them up for your cluster.
resource "helm_release" "exabot" {
  name             = "exabot-k8s"
  repository       = "https://exaforce.github.io/helm-charts"
  chart            = "exabot-k8s"
  namespace        = "exaforce"
  create_namespace = true

  set {
    name  = "exabotK8s.serviceAccount.roleArn"
    value = var.exabot_role_arn
  }

  set {
    name  = "exabotK8s.env.queueUrl"
    value = var.exabot_sqs_url
  }

  set {
    name  = "exabotK8s.env.configBucketId"
    value = var.bucket_id
  }
}

# Step 2: Create the CloudWatch subscription filter to forward audit logs
# to the ExaForce destination.
resource "aws_cloudwatch_log_subscription_filter" "exaforce" {
  name            = "exaforce-${var.cluster_name}-filter-${var.aws_region}"
  log_group_name  = "/aws/eks/${var.cluster_name}/cluster"
  filter_pattern  = "{ $.kind = \"*\" }"
  destination_arn = var.destination_arn
}

# Step 3: Register the EKS cluster as a log source in ExaForce CloudScout.
# Depends on the agent and subscription filter being in place first.
resource "exaforce_aws_logsource_eks" "cluster" {
  spec = {
    cluster_name = var.cluster_name
  }

  depends_on = [
    helm_release.exabot,
    aws_cloudwatch_log_subscription_filter.exaforce,
  ]
}
