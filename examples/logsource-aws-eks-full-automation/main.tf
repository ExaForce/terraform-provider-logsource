# Step 1: Enable all EKS control plane log types.
resource "aws_eks_cluster" "this" {
  name                      = var.cluster_name
  enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]
}

# Step 2: Create the CloudWatch subscription filter to forward audit logs
# to the ExaForce destination.
resource "aws_cloudwatch_log_subscription_filter" "exaforce" {
  name            = "exaforce-${var.cluster_name}"
  log_group_name  = "/aws/eks/${var.cluster_name}/cluster"
  filter_pattern  = "{ $.kind = \"*\" }"
  destination_arn = var.destination_arn

  depends_on = [aws_eks_cluster.this]
}

# Step 3: Install the ExaForce agent on the cluster.
# Values (role ARN, SQS URL, bucket ID) are obtained from CloudScout
# and passed in as variables — use the exaforce_aws_eks_clusters data
# source or the CloudScout UI to look them up for your cluster.
resource "helm_release" "exabot" {
  name             = "exabot-k8s"
  repository       = "https://exaforce.github.io/helm-charts"
  chart            = "exabot-k8s"
  namespace        = "exaforce"
  create_namespace = true

  set = [
    {
      name  = "exabotK8s.serviceAccount.roleArn"
      value = var.exabot_role_arn
    },
    {
      name  = "exabotK8s.env.queueUrl"
      value = var.exabot_sqs_url
    },
    {
      name  = "exabotK8s.env.configBucketId"
      value = var.bucket_id
    },
  ]

  depends_on = [aws_eks_cluster.this]
}

# Step 3.5: Wait for the subscription filter to propagate before registering
# the cluster. AWS takes a few seconds to make a newly created subscription
# filter visible to external readers.
# Only define this resource if you manage the subscription filter in this
# Terraform configuration. If the filter was created outside of Terraform
# (e.g. via CLI or a separate Terraform apply), remove this resource and
# the reference to it in step 4's depends_on.
resource "time_sleep" "after_subscription_filter" {
  create_duration = "20s"
  depends_on      = [aws_cloudwatch_log_subscription_filter.exaforce]
}

# Step 4: Register the EKS cluster as a log source in ExaForce CloudScout.
resource "exaforce_aws_logsource_eks" "cluster" {
  spec = {
    cluster_name = var.cluster_name
  }

  depends_on = [
    helm_release.exabot,
    time_sleep.after_subscription_filter,
  ]
}
