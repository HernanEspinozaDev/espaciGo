output "cloudsql_instance_name" {
  value       = try(google_sql_database_instance.scratch[0].name, null)
  description = "Only this run-scoped PostgreSQL 18 instance is managed by this Terraform root."
}

output "bucket_name" {
  value       = try(google_storage_bucket.private[0].name, null)
  description = "Run-scoped private bucket for the test manifest and synthetic evidence."
}

output "job_name" {
  value       = try(google_cloud_run_v2_job.smoke[0].name, null)
  description = "One-off smoke Job; no API service or recurring schedule is created."
}

output "runner_service_account" {
  value       = try(google_service_account.runner[0].email, null)
  description = "Dedicated identity with instance-scoped Cloud SQL and bucket/secret-scoped access."
}

output "artifact_repository" {
  value       = try(google_artifact_registry_repository.smoke[0].name, null)
  description = "Run-scoped repository for the smoke image digest."
}
