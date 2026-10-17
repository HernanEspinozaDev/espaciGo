output "origin_instance_name" {
  description = "Run-scoped private Cloud SQL instance name; null while Foundation is disabled."
  value       = try(google_sql_database_instance.origin[0].name, null)
}

output "origin_database_name" {
  description = "Empty run-scoped database name; null while Foundation is disabled."
  value       = try(google_sql_database.origin[0].name, null)
}

output "artifact_bucket_name" {
  description = "Private run-scoped bucket; null while Foundation is disabled."
  value       = try(google_storage_bucket.artifacts[0].name, null)
}

output "admin_service_account_email" {
  description = "Run-scoped bootstrap identity; null while Foundation is disabled."
  value       = try(google_service_account.admin[0].email, null)
}

output "admin_password_secret_id" {
  description = "Secret metadata only, with no payload/version; null while Foundation is disabled."
  value       = try(google_secret_manager_secret.admin_password[0].secret_id, null)
}
