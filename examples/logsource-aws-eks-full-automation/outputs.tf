output "logsource_name" {
  description = "ExaForce log source name assigned to the cluster"
  value       = exaforce_aws_logsource_eks.cluster.name
}

output "helm_install_command" {
  description = "Equivalent helm install command for reference or manual retry"
  value = join("\n", [
    "helm repo add exaforce https://exaforce.github.io/helm-charts",
    "helm repo update",
    "",
    join(" \\\n  ", [
      "helm install exabot-k8s exaforce/exabot-k8s",
      "--namespace exaforce --create-namespace",
      "--set exabotK8s.serviceAccount.roleArn=\"${var.exabot_role_arn}\"",
      "--set exabotK8s.env.queueUrl=\"${var.exabot_sqs_url}\"",
      "--set exabotK8s.env.configBucketId=\"${var.bucket_id}\"",
    ]),
  ])
}
