# Existing SMTP secret is managed separately. Bind only metadata visibility
# for the release gate: never create a container/version or grant payload access.
variable "enable_smtp_metadata_preflight" {
  type        = bool
  description = "Reviewed opt-in: allow Cloud Build to inspect metadata/IAM on the existing SMTP secret before rollout; no payload access."
  default     = false
}

resource "google_secret_manager_secret_iam_member" "smtp_preflight_viewer" {
  count     = var.enable_foundation && var.enable_smtp_metadata_preflight ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-test-auth-smtp-password"
  role      = "roles/secretmanager.viewer"
  member    = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}
