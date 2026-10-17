mock_provider "google" {
  override_during = plan
  mock_data "google_compute_network" {
    defaults = { self_link = "projects/espacigo-testprod/global/networks/espacigo-testprod" }
  }
  mock_data "google_compute_subnetwork" {
    defaults = { private_ip_google_access = true, region = "southamerica-west1" }
  }
}

run "foundation_is_off_by_default" {
  command = plan

  assert {
    condition = (
      length(google_sql_database_instance.origin) == 0 &&
      length(google_sql_database.origin) == 0 &&
      length(google_storage_bucket.artifacts) == 0 &&
      length(google_service_account.admin) == 0 &&
      length(google_secret_manager_secret.admin_password) == 0 &&
      length(google_secret_manager_secret_iam_member.admin_password_reader) == 0 &&
      length(google_project_iam_member.admin_sql_client) == 0
    )
    error_message = "No origin rehearsal resource may be planned while Foundation is off."
  }
}

run "foundation_is_private_protected_and_run_scoped" {
  command = plan

  variables {
    project_id               = "espacigo-testprod"
    region                   = "southamerica-west1"
    run_id                   = "g03o-1017"
    enable_foundation        = true
    confirm_origin_rehearsal = true
  }

  assert {
    condition = (
      google_sql_database_instance.origin[0].name == "espacigo-gcp03-origin-g03o-1017-pg" &&
      google_sql_database_instance.origin[0].database_version == "POSTGRES_18" &&
      google_sql_database_instance.origin[0].deletion_protection &&
      google_sql_database_instance.origin[0].settings[0].deletion_protection_enabled &&
      google_sql_database_instance.origin[0].settings[0].connector_enforcement == "REQUIRED" &&
      google_sql_database_instance.origin[0].settings[0].data_api_access == "DISALLOW_DATA_API" &&
      google_sql_database_instance.origin[0].settings[0].data_disk_provisioned_iops == 4000 &&
      google_sql_database_instance.origin[0].settings[0].data_disk_provisioned_throughput == 170 &&
      google_sql_database_instance.origin[0].settings[0].ip_configuration[0].ipv4_enabled == false &&
      google_sql_database_instance.origin[0].settings[0].ip_configuration[0].ssl_mode == "ENCRYPTED_ONLY" &&
      google_storage_bucket.artifacts[0].name == "espacigo-gcp03-origin-g03o-1017" &&
      google_storage_bucket.artifacts[0].force_destroy == false &&
      google_storage_bucket.artifacts[0].public_access_prevention == "enforced" &&
      google_storage_bucket.artifacts[0].soft_delete_policy[0].retention_duration_seconds == 0 &&
      google_secret_manager_secret.admin_password[0].deletion_protection &&
      google_secret_manager_secret.admin_password[0].deletion_policy == "PREVENT"
    )
    error_message = "Foundation must create only a new, private, protected, run-scoped target and empty secret metadata."
  }
}

run "foundation_requires_explicit_isolation_confirmation" {
  command = plan

  variables {
    run_id                   = "g03o-1017"
    enable_foundation        = true
    confirm_origin_rehearsal = false
  }

  expect_failures = [var.enable_foundation]
}
