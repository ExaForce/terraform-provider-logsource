output "helm_install_commands" {
  description = "Run these commands to install the ExaForce agent on each cluster"
  value = {
    for c in data.exaforce_aws_eks_clusters.all.clusters :
    c.name => join("\n", [
      "helm repo add exaforce https://exaforce.github.io/helm-charts",
      "helm repo update",
      "",
      join(" \\\n  ", [
        "helm install exabot-k8s exaforce/exabot-k8s",
        "--namespace exaforce --create-namespace",
        "--set exabotK8s.serviceAccount.roleArn=\"${c.exabot_role_arn}\"",
        "--set exabotK8s.env.queueUrl=\"${c.exabot_sqs_url}\"",
        "--set exabotK8s.env.configBucketId=\"${c.bucket_id}\"",
      ]),
    ])
    if contains(local.target_clusters, c.name)
  }
}
