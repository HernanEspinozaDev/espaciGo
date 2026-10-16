output "scratch_instance_name" {
  description = "Disposable Cloud SQL instance managed only by this root."
  value       = try(google_sql_database_instance.scratch[0].name, null)
}

output "scratch_database_name" {
  value = try(google_sql_database.scratch[0].name, null)
}

output "scratch_job_name" {
  value = try(google_cloud_run_v2_job.migrator["migrate"].name, null)
}

output "scratch_job_phase_names" {
  description = "Named bootstrap, database ACL, migrate, finalize and retire Jobs, created only when the separate job gate is enabled."
  value       = { for phase, job in google_cloud_run_v2_job.migrator : phase => job.name }
}

output "scratch_observer_job_name" {
  description = "Separate fixed observer Job; present only when enable_observer_job is explicitly true."
  value       = try(google_cloud_run_v2_job.observer[0].name, null)
}

output "scratch_owner_history_grant_job_name" {
  description = "Separate fixed migrator Job that grants postgres SELECT only on public.schema_migrations after history verification."
  value       = try(google_cloud_run_v2_job.owner_history_grant[0].name, null)
}

output "scratch_migrator_service_account" {
  value = try(google_service_account.scratch_migrator[0].email, null)
}

output "scratch_database_acl_service_account" {
  value = try(google_service_account.scratch_database_acl[0].email, null)
}

output "scratch_image_repository" {
  value = try(google_artifact_registry_repository.scratch[0].repository_id, null)
}
