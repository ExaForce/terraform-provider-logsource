output "logsource_name" {
  description = "ExaForce log source name assigned to the cluster"
  value       = exaforce_aws_logsource_eks.cluster.name
}
