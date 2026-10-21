mock_provider "google" {}

run "disabled_by_default" {
  command = plan

  assert {
    condition     = var.enable_bootstrap == false
    error_message = "Temporary bootstrap IAM must be disabled by default."
  }

  assert {
    condition     = length(google_iam_workload_identity_pool.bootstrap) == 0 && length(google_iam_workload_identity_pool_provider.bootstrap) == 0
    error_message = "Disabled bootstrap must create no WIF pool or provider."
  }

  assert {
    condition     = length(google_project_iam_custom_role.iam_role_metadata_reader) == 0 && length(google_project_iam_member.iam_role_metadata_reader) == 0
    error_message = "Disabled bootstrap must create no temporary IAM role metadata permission or binding."
  }

  assert {
    condition     = length(google_project_iam_custom_role.dependency_metadata_reader) == 0 && length(google_project_iam_member.dependency_metadata_reader) == 0 && length(google_project_iam_custom_role.runtime_subnetwork_metadata_reader) == 0 && length(google_compute_subnetwork_iam_member.runtime_subnetwork_metadata_reader) == 0
    error_message = "Disabled bootstrap must create no temporary dependency metadata permissions or bindings."
  }

  assert {
    condition     = length(google_project_iam_custom_role.deployer_service_account_reader) == 0 && length(google_service_account_iam_member.deployer_service_account_reader) == 0
    error_message = "Disabled bootstrap must create no temporary deployer service-account read role or binding."
  }

  assert {
    condition     = length(local.bootstrap_pool_display_name) <= local.wif_display_name_max_chars && length(local.bootstrap_provider_display_name) <= local.wif_display_name_max_chars
    error_message = "Pool and provider display names must respect the Google API's 32-character limit."
  }
}

run "bootstrap_access_is_constrained_and_expires" {
  command = plan

  override_resource {
    target          = google_iam_workload_identity_pool.bootstrap[0]
    override_during = plan
    values = {
      name = "projects/447373903153/locations/global/workloadIdentityPools/espacigo-gcp08-bootstrap"
    }
  }

  override_resource {
    target          = google_project_iam_custom_role.iam_role_metadata_reader[0]
    override_during = plan
    values = {
      name = "projects/espacigo-testprod/roles/espacigoGcp08IamRoleReader"
    }
  }

  override_resource {
    target          = google_project_iam_custom_role.deployer_service_account_reader[0]
    override_during = plan
    values = {
      name = "projects/espacigo-testprod/roles/espacigoGcp08DeployerServiceAccountReader"
    }
  }

  override_resource {
    target          = google_project_iam_custom_role.runtime_service_account_user[0]
    override_during = plan
    values = {
      name = "projects/espacigo-testprod/roles/espacigoGcp08RuntimeServiceAccountUser"
    }
  }

  variables {
    enable_bootstrap     = true
    bootstrap_expires_at = timeadd(timestamp(), "1h")
  }

  assert {
    condition     = google_iam_workload_identity_pool.bootstrap[0].display_name == local.bootstrap_pool_display_name && length(google_iam_workload_identity_pool.bootstrap[0].display_name) <= 32 && google_iam_workload_identity_pool_provider.bootstrap[0].display_name == local.bootstrap_provider_display_name && length(google_iam_workload_identity_pool_provider.bootstrap[0].display_name) <= 32
    error_message = "Rendered WIF pool and provider display names must each stay within the API's 32-character limit."
  }

  assert {
    condition = toset(google_project_iam_custom_role.cloud_run_bootstrap[0].permissions) == toset([
      "run.operations.get",
      "run.services.create",
      "run.services.get",
      "run.services.getIamPolicy",
      "run.services.list",
      "run.services.setIamPolicy",
      "run.services.update",
    ])
    error_message = "Temporary Cloud Run bootstrap must contain only the seven reviewed service permissions."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.iam_role_metadata_reader[0].permissions) == toset(["iam.roles.get"])
    error_message = "The separate temporary IAM role metadata role must contain only iam.roles.get."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.dependency_metadata_reader[0].permissions) == toset(["compute.networks.get", "resourcemanager.projects.get", "serviceusage.services.get"]) && google_project_iam_member.dependency_metadata_reader[0].member == local.workload_member && google_project_iam_member.dependency_metadata_reader[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}')"
    error_message = "Project-scoped dependency metadata access must contain only the observed project/network/API read permissions and expire with the bootstrap; project IAM policy read stays excluded."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.runtime_subnetwork_metadata_reader[0].permissions) == toset(["compute.subnetworks.get"]) && toset(keys(google_compute_subnetwork_iam_member.runtime_subnetwork_metadata_reader)) == toset(local.runtime_subnetwork_names) && alltrue([for grant in values(google_compute_subnetwork_iam_member.runtime_subnetwork_metadata_reader) : grant.member == local.workload_member && grant.region == var.region && grant.condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}')"])
    error_message = "Subnetwork metadata read must expire and be bound only to the two existing serverless subnetworks."
  }

  assert {
    condition     = google_project_iam_member.iam_role_metadata_reader[0].role == google_project_iam_custom_role.iam_role_metadata_reader[0].name && google_project_iam_member.iam_role_metadata_reader[0].member == local.workload_member && google_project_iam_member.iam_role_metadata_reader[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}')"
    error_message = "IAM role metadata read must go only to the exact bootstrap workflow principal and share its expiry; it must not depend on IAM resource name attributes."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.runtime_service_account_user[0].permissions) == toset(["iam.serviceAccounts.actAs", "iam.serviceAccounts.get", "iam.serviceAccounts.getIamPolicy"])
    error_message = "The three runtime service accounts may receive only actAs, metadata get, and IAM-policy get."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.deployer_service_account_reader[0].permissions) == toset(["iam.serviceAccounts.get", "iam.serviceAccounts.getIamPolicy"])
    error_message = "The deployer service account read role must contain exactly get and getIamPolicy, with no write permission."
  }

  assert {
    condition     = google_iam_workload_identity_pool_provider.bootstrap[0].attribute_condition == "assertion.repository == 'HernanEspinozaDev/espaciGo' && assertion.repository_id == '1361448198' && assertion.repository_owner_id == '176093371' && assertion.actor_id == '176093371' && assertion.ref == 'refs/heads/main' && assertion.workflow_ref == 'HernanEspinozaDev/espaciGo/.github/workflows/application-rollout.yml@refs/heads/main' && assertion.event_name == 'workflow_dispatch' && assertion.environment == 'testprod'"
    error_message = "Temporary WIF must accept only the verified actor and exact manual main rollout under the protected testprod environment."
  }

  assert {
    condition     = google_project_iam_member.cloud_run_bootstrap[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}') && api.getAttribute('iam.googleapis.com/modifiedGrantsByRole', []).hasOnly(${jsonencode(local.bootstrap_cloud_run_iam_roles)})"
    error_message = "Temporary Cloud Run access must expire and may change only the two approved service IAM roles."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.v67_cloudsql_metadata_reader[0].permissions) == toset(["cloudsql.instances.get"])
    error_message = "Bootstrap may read only the named Cloud SQL instance metadata; database metadata is pinned by validated configuration."
  }

  assert {
    condition     = google_project_iam_member.v67_cloudsql_metadata_reader[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}') && resource.type == 'sqladmin.googleapis.com/Instance' && resource.name == 'projects/espacigo-testprod/instances/espacigo-testprod-db'"
    error_message = "Cloud SQL metadata read must expire and be limited to the named V67 instance."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.v67_secret_metadata_reader[0].permissions) == toset(["secretmanager.secrets.get", "secretmanager.secrets.getIamPolicy", "secretmanager.secrets.setIamPolicy"]) && google_secret_manager_secret_iam_member.v67_secret_metadata_reader[0].secret_id == "espacigo-test-database-password-v67" && google_secret_manager_secret_iam_member.v67_secret_metadata_reader[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}') && resource.type == 'secretmanager.googleapis.com/Secret' && resource.name == 'projects/447373903153/secrets/espacigo-test-database-password-v67'"
    error_message = "Bootstrap may read metadata and manage the allowlisted IAM member only on the exact V67 secret, and only until expiry."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.private_iap_policy_manager[0].permissions) == toset(["iap.webServices.getIamPolicy", "iap.webServices.setIamPolicy"]) && google_project_iam_member.private_iap_policy_manager[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}') && resource.type == 'iap.googleapis.com/WebService'"
    error_message = "Bootstrap IAP policy permissions must expire and apply only to IAP WebService resources."
  }

  assert {
    condition     = toset(keys(google_service_account_iam_member.runtime_service_account_user)) == toset(local.runtime_service_account_names) && alltrue([for account_name, grant in google_service_account_iam_member.runtime_service_account_user : grant.service_account_id == account_name && grant.role == google_project_iam_custom_role.runtime_service_account_user[0].name && grant.member == local.workload_member && grant.condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}')"])
    error_message = "The runtime role must be directly bound only to the three runtime accounts and use the unchanged bootstrap expiry."
  }

  assert {
    condition     = google_service_account_iam_member.deployer_service_account_reader[0].service_account_id == local.deployer_service_account_name && google_service_account_iam_member.deployer_service_account_reader[0].role == google_project_iam_custom_role.deployer_service_account_reader[0].name && google_service_account_iam_member.deployer_service_account_reader[0].member == local.workload_member && google_service_account_iam_member.deployer_service_account_reader[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}')"
    error_message = "The read-only deployer role must bind directly to only the deployer account and share the existing bootstrap expiry."
  }

  assert {
    condition     = google_storage_managed_folder_iam_member.terraform_state[0].managed_folder == "espacigo/testprod/gcp/" && google_storage_managed_folder_iam_member.terraform_state[0].role == "projects/espacigo-testprod/roles/espacigoTerraformStateObjectAccess"
    error_message = "Temporary state access must stay within the canonical GCP state managed folder."
  }

  assert {
    condition     = terraform_data.bootstrap_expiry_guard[0].input == var.bootstrap_expires_at
    error_message = "The apply-time 24-hour expiry guard must be part of the bootstrap plan."
  }
}
