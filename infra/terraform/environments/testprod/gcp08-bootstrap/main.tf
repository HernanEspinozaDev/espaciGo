locals {
  workflow_ref                    = "${var.github_repository}/.github/workflows/application-rollout.yml@refs/heads/main"
  workload_pool_name              = try(google_iam_workload_identity_pool.bootstrap[0].name, "")
  workload_member                 = "principalSet://iam.googleapis.com/${local.workload_pool_name}/attribute.workflow_ref/${local.workflow_ref}"
  bootstrap_expiry                = "request.time < timestamp('${var.bootstrap_expires_at}')"
  wif_display_name_max_chars      = 32
  bootstrap_pool_display_name     = "EspaciGo GCP08 bootstrap"
  bootstrap_provider_display_name = "EspaciGo GCP08 restore"
  bootstrap_cloud_run_iam_roles = [
    "roles/run.invoker",
    "projects/${var.project_id}/roles/espacigoCloudRunServiceUpdater",
  ]
  bootstrap_v67_instance_name   = "projects/${var.project_id}/instances/espacigo-testprod-db"
  bootstrap_v67_secret_name     = "projects/${var.project_number}/secrets/espacigo-test-database-password-v67"
  deployer_service_account_name = "projects/${var.project_id}/serviceAccounts/espacigo-test-deployer@${var.project_id}.iam.gserviceaccount.com"
  runtime_service_account_names = [
    "projects/${var.project_id}/serviceAccounts/espacigo-test-api@${var.project_id}.iam.gserviceaccount.com",
    "projects/${var.project_id}/serviceAccounts/espacigo-test-public-web@${var.project_id}.iam.gserviceaccount.com",
    "projects/${var.project_id}/serviceAccounts/espacigo-test-private-web@${var.project_id}.iam.gserviceaccount.com",
  ]
  runtime_subnetwork_names = [
    "espacigo-testprod-serverless",
    "espacigo-testprod-api-serverless",
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

    precondition {
      condition = (
        length(local.bootstrap_pool_display_name) <= local.wif_display_name_max_chars &&
        length(local.bootstrap_provider_display_name) <= local.wif_display_name_max_chars
      )
      error_message = "Google Workload Identity pool and provider display names must each be 32 characters or fewer."
    }
  }
}

resource "google_iam_workload_identity_pool" "bootstrap" {
  count                     = var.enable_bootstrap ? 1 : 0
  project                   = var.project_id
  workload_identity_pool_id = "espacigo-gcp08-bootstrap"
  display_name              = local.bootstrap_pool_display_name
  description               = "Short-lived direct federation for the protected GCP-08 initial restore workflow."
  disabled                  = false

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_iam_workload_identity_pool_provider" "bootstrap" {
  count                              = var.enable_bootstrap ? 1 : 0
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.bootstrap[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "espacigo-initial-restore"
  display_name                       = local.bootstrap_provider_display_name

  attribute_mapping = {
    "google.subject"                = "assertion.sub"
    "attribute.repository"          = "assertion.repository"
    "attribute.repository_id"       = "assertion.repository_id"
    "attribute.repository_owner_id" = "assertion.repository_owner_id"
    "attribute.ref"                 = "assertion.ref"
    "attribute.workflow_ref"        = "assertion.workflow_ref"
    "attribute.environment"         = "assertion.environment"
    "attribute.event_name"          = "assertion.event_name"
    "attribute.actor_id"            = "assertion.actor_id"
  }

  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_repository_owner_id}' && assertion.actor_id == '${var.github_bootstrap_actor_id}' && assertion.ref == 'refs/heads/main' && assertion.workflow_ref == '${local.workflow_ref}' && assertion.event_name == 'workflow_dispatch' && assertion.environment == 'testprod'"

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
  description = "Temporary create/list/read/update/IAM-policy access for the protected initial restore; excludes delete, Jobs, project IAM and other services."
  stage       = "GA"
  permissions = [
    "run.operations.get",
    "run.services.create",
    "run.services.get",
    "run.services.getIamPolicy",
    "run.services.list",
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

resource "google_project_iam_custom_role" "iam_role_metadata_reader" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08IamRoleReader"
  title       = "EspaciGo GCP-08 temporary IAM role metadata read"
  description = "Temporary read-only access to IAM role definitions needed by the initial Cloud Run restore plan."
  stage       = "GA"
  permissions = ["iam.roles.get"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_member" "iam_role_metadata_reader" {
  count   = var.enable_bootstrap ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.iam_role_metadata_reader[0].name
  member  = local.workload_member

  condition {
    title       = "gcp08-iam-role-read-expiry"
    description = "Expires automatically with the GCP-08 bootstrap window; read-only IAM role metadata access."
    expression  = local.bootstrap_expiry
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "dependency_metadata_reader" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08DependencyReader"
  title       = "EspaciGo GCP-08 temporary dependency metadata read"
  description = "Temporary read-only access to existing project, network, and enabled API metadata needed by the protected restore plan."
  stage       = "GA"
  permissions = ["compute.networks.get", "resourcemanager.projects.get", "serviceusage.services.get", "serviceusage.services.list"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_member" "dependency_metadata_reader" {
  count   = var.enable_bootstrap ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.dependency_metadata_reader[0].name
  member  = local.workload_member

  condition {
    title       = "gcp08-dependency-metadata-expiry"
    description = "Expires with the protected initial-restore bootstrap window; grants only the four read permissions required for project, network, and service usage metadata refresh."
    expression  = local.bootstrap_expiry
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "runtime_subnetwork_metadata_reader" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08SubnetworkReader"
  title       = "EspaciGo GCP-08 temporary subnetwork metadata read"
  description = "Temporary read-only access to the two existing serverless subnetworks used by the protected restore plan."
  stage       = "GA"
  permissions = ["compute.subnetworks.get"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_compute_subnetwork_iam_member" "runtime_subnetwork_metadata_reader" {
  for_each   = var.enable_bootstrap ? toset(local.runtime_subnetwork_names) : toset([])
  project    = var.project_id
  region     = var.region
  subnetwork = each.value
  role       = google_project_iam_custom_role.runtime_subnetwork_metadata_reader[0].name
  member     = local.workload_member

  condition {
    title       = "gcp08-subnetwork-metadata-expiry"
    description = "Expires with the bootstrap and grants read access on only the two existing serverless subnetworks."
    expression  = local.bootstrap_expiry
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "v67_cloudsql_metadata_reader" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08V67CloudSqlMetadataReader"
  title       = "EspaciGo GCP-08 temporary V67 Cloud SQL metadata read"
  description = "Read only metadata for the named V67 Cloud SQL instance during the bootstrap plan."
  stage       = "GA"
  permissions = ["cloudsql.instances.get"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_member" "v67_cloudsql_metadata_reader" {
  count   = var.enable_bootstrap ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.v67_cloudsql_metadata_reader[0].name
  member  = local.workload_member

  condition {
    title       = "gcp08-v67-cloudsql-metadata-expiry"
    description = "Expires within 24 hours and is restricted to the existing V67 SQL instance metadata."
    expression  = "${local.bootstrap_expiry} && resource.type == 'sqladmin.googleapis.com/Instance' && resource.name == '${local.bootstrap_v67_instance_name}'"
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "v67_secret_metadata_reader" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08V67SecretMetadataReader"
  title       = "EspaciGo GCP-08 temporary V67 secret metadata read"
  description = "Read metadata only for the single V67 runtime database secret; does not read secret versions or payloads."
  stage       = "GA"
  permissions = [
    "secretmanager.secrets.get",
    "secretmanager.secrets.getIamPolicy",
    "secretmanager.secrets.setIamPolicy",
  ]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_secret_manager_secret_iam_member" "v67_secret_metadata_reader" {
  count     = var.enable_bootstrap ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-test-database-password-v67"
  role      = google_project_iam_custom_role.v67_secret_metadata_reader[0].name
  member    = local.workload_member

  condition {
    title       = "gcp08-v67-secret-metadata-expiry"
    description = "Expires within 24 hours; grants metadata read on the single V67 runtime database secret only."
    expression  = "${local.bootstrap_expiry} && resource.type == 'secretmanager.googleapis.com/Secret' && resource.name == '${local.bootstrap_v67_secret_name}'"
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "private_iap_policy_manager" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08PrivateIapPolicyManager"
  title       = "EspaciGo GCP-08 temporary private IAP policy manager"
  description = "Temporary read/set access to IAP WebService policies; limited to that resource type and bounded by the bootstrap window."
  stage       = "GA"
  permissions = ["iap.webServices.getIamPolicy", "iap.webServices.setIamPolicy"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_member" "private_iap_policy_manager" {
  count   = var.enable_bootstrap ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.private_iap_policy_manager[0].name
  member  = local.workload_member

  condition {
    title       = "gcp08-private-iap-policy-expiry"
    description = "Expires within 24 hours and is limited to IAP WebService policy resources."
    expression  = "${local.bootstrap_expiry} && resource.type == 'iap.googleapis.com/WebService'"
  }

  depends_on = [google_iam_workload_identity_pool_provider.bootstrap, terraform_data.bootstrap_expiry_guard]
}

resource "google_project_iam_custom_role" "runtime_service_account_user" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08RuntimeServiceAccountUser"
  title       = "EspaciGo GCP-08 temporary runtime service account attachment"
  description = "Temporary actAs and read-only metadata/IAM-policy access for only the three runtime service accounts."
  stage       = "GA"
  permissions = [
    "iam.serviceAccounts.actAs",
    "iam.serviceAccounts.get",
    "iam.serviceAccounts.getIamPolicy",
  ]

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

resource "google_project_iam_custom_role" "deployer_service_account_reader" {
  count       = var.enable_bootstrap ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp08DeployerServiceAccountReader"
  title       = "EspaciGo GCP-08 temporary deployer service account read"
  description = "Temporary read-only metadata and IAM-policy access for only the deployer service account."
  stage       = "GA"
  permissions = ["iam.serviceAccounts.get", "iam.serviceAccounts.getIamPolicy"]

  depends_on = [terraform_data.bootstrap_expiry_guard]
}

resource "google_service_account_iam_member" "deployer_service_account_reader" {
  count              = var.enable_bootstrap ? 1 : 0
  service_account_id = local.deployer_service_account_name
  role               = google_project_iam_custom_role.deployer_service_account_reader[0].name
  member             = local.workload_member

  condition {
    title       = "gcp08-deployer-service-account-read-expiry"
    description = "Expires automatically with the bootstrap and is bound directly to only the deployer service account."
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
