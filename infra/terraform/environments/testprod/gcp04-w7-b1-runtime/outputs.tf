output "job_name" {
  value       = try(google_cloud_run_v2_job.probe[0].name, null)
  description = "Run-scoped Cloud Run Job name; null while the package is disabled."
}

output "service_account_email" {
  value       = try(google_service_account.runtime[0].email, null)
  description = "Dedicated runtime identity; null while the package is disabled."
}
