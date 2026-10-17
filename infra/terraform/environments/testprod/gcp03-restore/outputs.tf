output "restore_instance_name" {
  description = "Separate run-scoped Cloud SQL instance. Null while the gate is off."
  value       = try(google_sql_database_instance.restore[0].name, null)
}

output "restore_job_name" {
  description = "Single-shot pg_restore Job resource; Terraform creates it but does not execute it."
  value       = try(google_cloud_run_v2_job.restore[0].name, null)
}

output "bootstrap_job_name" {
  description = "Bootstrap Job resource, when separately enabled; Terraform never runs it."
  value       = try(google_cloud_run_v2_job.admin_phase["bootstrap"].name, null)
}

output "verify_job_name" {
  description = "Separate verifier Job resource; Terraform never runs it."
  value       = try(google_cloud_run_v2_job.verify[0].name, null)
}

output "diagnostic_job_name" {
  description = "Default-off read-only artifact and database-catalog diagnostic Job; Terraform creates it but never executes it."
  value       = try(google_cloud_run_v2_job.diagnose[0].name, null)
}

output "retire_job_name" {
  description = "Post-verify retirement Job resource; Terraform never runs it."
  value       = try(google_cloud_run_v2_job.admin_phase["retire"].name, null)
}

output "retirement_diagnostic_job_name" {
  description = "Default-off read-only catalog diagnosis Job using the existing run-scoped admin identity; Terraform creates it but never executes it."
  value       = try(google_cloud_run_v2_job.retirement_diagnose[0].name, null)
}

output "artifact_bucket_name" {
  description = "Private run-scoped synthetic artifact bucket."
  value       = try(google_storage_bucket.artifacts[0].name, null)
}
