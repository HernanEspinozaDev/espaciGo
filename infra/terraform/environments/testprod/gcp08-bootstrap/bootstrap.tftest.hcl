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
}

run "bootstrap_access_is_constrained_and_expires" {
  command = plan

  variables {
    enable_bootstrap     = true
    bootstrap_expires_at = timeadd(timestamp(), "1h")
  }

  assert {
    condition = toset(google_project_iam_custom_role.cloud_run_bootstrap[0].permissions) == toset([
      "run.operations.get",
      "run.services.create",
      "run.services.get",
      "run.services.getIamPolicy",
      "run.services.setIamPolicy",
      "run.services.update",
    ])
    error_message = "Temporary Cloud Run bootstrap must contain only the six reviewed service permissions."
  }

  assert {
    condition     = toset(google_project_iam_custom_role.runtime_service_account_user[0].permissions) == toset(["iam.serviceAccounts.actAs"])
    error_message = "Temporary service-account attachment access must contain only actAs."
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
    condition     = toset(google_project_iam_custom_role.v67_cloudsql_metadata_reader[0].permissions) == toset(["cloudsql.instances.get", "cloudsql.databases.get"])
    error_message = "Bootstrap may read only Cloud SQL instance and database metadata."
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
    condition     = toset(google_project_iam_custom_role.private_iap_policy_manager[0].permissions) == toset(["iap.webServices.getIamPolicy", "iap.webServices.setIamPolicy"]) && google_project_iam_member.private_iap_policy_manager[0].condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}') && resource.type == 'iap.googleapis.com/WebService' && resource.name == 'projects/447373903153/iap_web/cloud_run-southamerica-west1/services/espacigo-test-private'"
    error_message = "Bootstrap may read and manage IAP policy only on the exact private Cloud Run IAP resource, and only until expiry."
  }

  assert {
    condition     = toset(keys(google_service_account_iam_member.runtime_service_account_user)) == toset(local.runtime_service_account_names) && alltrue([for grant in values(google_service_account_iam_member.runtime_service_account_user) : grant.condition[0].expression == "request.time < timestamp('${var.bootstrap_expires_at}')"])
    error_message = "actAs must be granted directly on only the three runtime service accounts and expire with the bootstrap."
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
