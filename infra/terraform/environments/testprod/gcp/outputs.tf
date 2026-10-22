output "project_id" {
  value       = var.project_id
  description = "Existing project selected for this test environment."
}

output "region" {
  value       = var.region
  description = "Chosen Santiago region for the test environment."
}

output "load_balancer_ipv4" {
  value       = try(google_compute_global_address.edge[0].address, null)
  description = "Copy explicitly to the private Cloudflare configuration after DNS review."
}

output "mailpit_private_smtp_address" {
  value       = var.enable_mailpit ? "${google_compute_address.mailpit[0].address}:1025" : null
  description = "Private SMTP endpoint for API test traffic."
}

output "mailpit_instance_identity" {
  value       = var.enable_mailpit ? "${var.mailpit_zone}/${google_compute_instance.mailpit[0].name}" : null
  description = "Private VM identity used to open the IAP TCP tunnel to the Mailpit UI."
}

output "terraform_wif_provider" {
  value       = try(google_iam_workload_identity_pool_provider.github[0].name, null)
  description = "Legacy GitHub Actions provider retained until separately reviewed IAM cleanup."
}
