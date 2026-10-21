output "workload_identity_provider" {
  value = var.enable_bootstrap ? "projects/${var.project_number}/locations/global/workloadIdentityPools/${google_iam_workload_identity_pool.bootstrap[0].workload_identity_pool_id}/providers/${google_iam_workload_identity_pool_provider.bootstrap[0].workload_identity_pool_provider_id}" : null
}

output "expires_at" {
  value = var.enable_bootstrap ? var.bootstrap_expires_at : null
}
