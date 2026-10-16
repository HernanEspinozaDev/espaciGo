output "scratch_instance_name" {
  description = "Disposable Cloud SQL instance managed only by this root."
  value       = try(google_sql_database_instance.scratch[0].name, null)
}

output "scratch_database_name" {
  value = try(google_sql_database.scratch[0].name, null)
}

output "scratch_job_name" {
  value = try(google_cloud_run_v2_job.migrator[0].name, null)
}

output "scratch_migrator_service_account" {
  value = try(google_service_account.scratch_migrator[0].email, null)
}

output "scratch_image_repository" {
  value = try(google_artifact_registry_repository.scratch[0].repository_id, null)
}
