output "logsource_name" {
  description = "ExaForce log source name assigned to the cluster"
  value       = exaforce_aws_logsource_eks.this.name
}

output "cluster_info" {
  description = "CloudScout metadata for the onboarded cluster"
  value = {
    name            = local.cluster.name
    arn             = local.cluster.arn
    region          = local.cluster.region
    account_id      = local.cluster.account_id
    exabot_role_arn = local.cluster.exabot_role_arn
    exabot_sqs_url  = local.cluster.exabot_sqs_url
    bucket_id       = local.cluster.bucket_id
    destination_arn = local.cluster.destination_arn
  }
}
