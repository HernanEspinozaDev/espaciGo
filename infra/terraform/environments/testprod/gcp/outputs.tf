output "project_id" {
  value       = var.project_id
  description = "Existing project selected for this test environment."
}

output "region" {
  value       = var.region
  description = "Region selected for the test environment."
}

output "load_balancer_ipv4" {
  value       = try(google_compute_global_address.edge[0].address, null)
  description = "Copy explicitly to the private Cloudflare tfvars after inventory and DNS review."
}

output "private_iap_audience" {
  value       = var.enable_services ? "/projects/${var.project_number}/locations/${var.region}/services/${var.private_service_name}" : null
  description = "Direct Cloud Run IAP audience; backendServices audiences are intentionally not configured."
}

output "api_service_audience" {
  value       = var.enable_services ? google_cloud_run_v2_service.api[0].uri : null
  description = "Cloud Run service origin used only as the IAM ID-token audience."
}

output "api_internal_base_url" {
  value       = var.enable_services ? "${google_cloud_run_v2_service.api[0].uri}/api/v1" : null
  description = "Backend API base path consumed by SSR and accepted by configuredApiBase."
}

output "public_web_runapp_url" {
  value       = var.enable_services ? google_cloud_run_v2_service.web["public"].uri : null
  description = "Cloud Run URI for the public web service; direct external access is available when enable_edge is false."
}

output "private_web_runapp_url" {
  value       = var.enable_services ? google_cloud_run_v2_service.web["private"].uri : null
  description = "Cloud Run URI for the native-IAP private web service; direct external access is available when enable_edge is false."
}

output "mailpit_private_smtp_address" {
  value       = var.enable_mailpit ? "${google_compute_address.mailpit[0].address}:1025" : null
  description = "Private SMTP endpoint for API test traffic; firewall permits only the dedicated API subnet."
}

output "mailpit_instance_identity" {
  value       = var.enable_mailpit ? "${var.mailpit_zone}/${google_compute_instance.mailpit[0].name}" : null
  description = "Private VM identity used to open an IAP TCP tunnel to the Mailpit UI on port 8025."
}

output "cloudsql_connection_name" {
  value       = try(google_sql_database_instance.postgres[0].connection_name, null)
  description = "Cloud SQL Connector connection name."
}

output "terraform_wif_provider" {
  value       = try(google_iam_workload_identity_pool_provider.github[0].name, null)
  description = "Repository/ref-restricted GitHub OIDC provider."
}
