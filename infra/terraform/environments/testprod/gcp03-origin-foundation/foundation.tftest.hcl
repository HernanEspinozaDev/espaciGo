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
      length(google_project_iam_member.admin_sql_client) == 0 &&
      length(google_cloud_run_v2_job.origin_bootstrap) == 0 &&
      length(google_cloud_run_v2_job.origin_observer) == 0 &&
      length(google_service_account.origin_observer) == 0 &&
      length(google_sql_user.origin_observer) == 0
    )
    error_message = "No origin rehearsal resource may be planned while Foundation is off."
  }
}

run "bootstrap_job_is_one_shot_and_never_manages_its_secret_version" {
  command = plan

  variables {
    run_id                                = "g03o-1017"
    enable_foundation                     = true
    confirm_origin_rehearsal              = true
    enable_origin_bootstrap_job           = true
    origin_executor_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    admin_password_secret_version         = "3"
    confirm_admin_password_secret_version = true
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.origin_bootstrap) == 1 &&
      google_cloud_run_v2_job.origin_bootstrap[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.origin_bootstrap[0].template[0].parallelism == 1 &&
      google_cloud_run_v2_job.origin_bootstrap[0].template[0].template[0].max_retries == 0 &&
      length(google_secret_manager_secret.admin_password) == 1 &&
      google_cloud_run_v2_job.origin_bootstrap[0].template[0].template[0].volumes[0].secret[0].items[0].version == "3"
    )
    error_message = "Bootstrap must be a one-task, zero-retry Job using the admin identity and a pre-existing numeric secret version."
  }
}

run "observer_job_is_iam_only_and_has_no_secret_or_bucket_access" {
  command = plan

  variables {
    run_id                             = "g03o-1017"
    enable_foundation                  = true
    confirm_origin_rehearsal           = true
    enable_origin_observer_job         = true
    confirm_origin_bootstrap_completed = true
    bootstrap_evidence_sha256          = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    origin_executor_image              = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.origin_observer) == 1 &&
      google_cloud_run_v2_job.origin_observer[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.origin_observer[0].template[0].parallelism == 1 &&
      google_cloud_run_v2_job.origin_observer[0].template[0].template[0].max_retries == 0 &&
      length(google_cloud_run_v2_job.origin_observer[0].template[0].template[0].volumes) == 0 &&
      length(google_secret_manager_secret_iam_member.admin_password_reader) == 1 &&
      length(google_storage_bucket.artifacts) == 1 &&
      length(google_sql_user.origin_observer[0].database_roles) == 1 &&
      contains(google_sql_user.origin_observer[0].database_roles, "espacigo_gcp03_observer")
    )
    error_message = "Observer must use a distinct IAM identity, no secret mount, and only its fixed database role."
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
