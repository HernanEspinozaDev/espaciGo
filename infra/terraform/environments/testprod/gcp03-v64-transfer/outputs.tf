output "transfer_job_name" {
  value       = try(google_cloud_run_v2_job.transfer[0].name, null)
  description = "The default-off one-shot transfer Job; Terraform never executes it."
}

output "transfer_service_account" {
  value       = try(google_service_account.transfer[0].email, null)
  description = "Run-scoped identity for the transfer Job."
}
