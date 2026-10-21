locals {
  workflow_ref       = "${var.github_repository}/.github/workflows/application-rollout.yml@refs/heads/main"
  workload_pool_name = try(google_iam_workload_identity_pool.bootstrap[0].name, "")
  workload_member    = "principalSet://iam.googleapis.com/${local.workload_pool_name}/attribute.workflow_ref/${local.workflow_ref}"
  bootstrap_expiry   = "request.time < timestamp('${var.bootstrap_expires_at}')"
  bootstrap_cloud_run_iam_roles = [
    "roles/run.invoker",
    "projects/${var.project_id}/roles/espacigoCloudRunServiceUpdater",
  ]
  runtime_service_account_names = [
    "projects/${var.project_id}/serviceAccounts/espacigo-test-api@${var.project_id}.iam.gserviceaccount.com",
    "projects/${var.project_id}/serviceAccounts/espacigo-test-public-web@${var.project_id}.iam.gserviceaccount.com",
    "projects/${var.project_id}/serviceAccounts/espacigo-test-private-web@${var.project_id}.iam.gserviceaccount.com",
  ]
}

resource "terraform_data" "bootstrap_expiry_guard" {
  count = var.enable_bootstrap ? 1 : 0
  input = var.bootstrap_expires_at

  lifecycle {
    precondition {
      condition = (
        can(timecmp(var.bootstrap_expires_at, timestamp())) &&
        timecmp(var.bootstrap_expires_at, timestamp()) > 0 &&
        timecmp(var.bootstrap_expires_at, timeadd(timestamp(), "24h")) <= 0
      )
      error_message = "Bootstrap IAM expiration must be in the future and within 24 hours at apply time. Re-plan with a new expiration."
    }
  }
}

resource "google_iam_workload_identity_pool" "bootstrap" {
  count                     = var.enable_bootstrap ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "espacigo-gcp08-bootstrap"
  display_name              = "EspaciGo GCP-08 one-time Cloud Run restore"
  description               = "Short-lived direct federation for the protected GCP-08 initial restore workflow."
  disabled                  = false

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_iam_workload_identity_pool_provider" "bootstrap" {
  count                              = var.enable_bootstrap ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.bootstrap[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "espacigo-initial-restore"
  display_name                       = "Protected GCP-08 initial restore workflow"

  attribute_mapping = {
    "google.subject"                = "assertion.sub"
    "attribute.repository"          = "assertion.repository"
    "attribute.repository_id"       = "assertion.repository_id"
    "attribute.repository_owner_id" = "assertion.repository_owner_id"
    "attribute.ref"                 = "assertion.ref"
    "attribute.workflow_ref"        = "assertion.workflow_ref"
    "attribute.environment"         = "assertion.environment"
    "attribute.event_name"          = "assertion.event_name"
  }

  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_repository_owner_id}' && assertion.ref == 'refs/heads/main' && assertion.workflow_ref == '${local.workflow_ref}' && assertion.event_name == 'workflow_dispatch' && assertion.environment == 'testprod'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "cloud_run_bootstrap" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08CloudRunBootstrap"
  title       = "EspaciGo GCP-08 temporary Cloud Run bootstrap"
  description = "Temporary create/read/update/IAM-policy access for the protected initial restore; excludes delete, Jobs, project IAM and other services."
  stage       = "GA"
  permissions = [
    "run.operations.get",
    "run.services.create",
    "run.services.get",
    "run.services.getIamPolicy",
    "run.services.setIamPolicy",
    "run.services.update",
  ]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_member" "cloud_run_bootstrap" {
  count   = var.enable_bootstrap ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.cloud_run_bootstrap[0].name
  member  = local.workload_member

  condition {
    title       = "gcp08-initial-restore-expiry"
    description = "Expires automatically within 24 hours; only the separate protected restore workflow receives this role."
    expression  = "${local.bootstrap_expiry} && api.getAttribute('iam.googleapis.com/modifiedGrantsByRole', []).hasOnly(${jsonencode(local.bootstrap_cloud_run_iam_roles)})"
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "runtime_service_account_user" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08RuntimeServiceAccountUser"
  title       = "EspaciGo GCP-08 temporary runtime service account attachment"
  description = "Temporary iam.serviceAccounts.actAs for only the API and two web runtime service accounts."
  stage       = "GA"
  permissions = ["iam.serviceAccounts.actAs"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_service_account_iam_member" "runtime_service_account_user" {
  for_each           = var.enable_bootstrap ? toset(local.runtime_service_account_names) : toset([])
  service_account_id = each.value
  role               = google_project_iam_custom_role.runtime_service_account_user[0].name
  member             = local.workload_member

  condition {
    title       = "gcp08-runtime-service-account-expiry"
    description = "Expires automatically within 24 hours and is limited to the three Cloud Run runtime identities."
    expression  = local.bootstrap_expiry
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_storage_managed_folder_iam_member" "terraform_state" {
  count          = var.enable_bootstrap ? 1 : 0
  bucket         = "espacigo-testprod-tfstate-447373903153-gcp"
  managed_folder = "espacigo/testprod/gcp/"
  role           = "projects/${var.project_id}/roles/espacigoTerraformStateObjectAccess"
  member         = local.workload_member

  condition {
    title       = "gcp08-terraform-state-expiry"
    description = "Expires automatically within 24 hours; grants object access only below the canonical testprod/gcp state folder."
    expression  = local.bootstrap_expiry
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}
