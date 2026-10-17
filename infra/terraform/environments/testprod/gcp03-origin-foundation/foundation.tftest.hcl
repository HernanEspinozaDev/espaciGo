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
      && length(google_cloud_run_v2_job.origin_import) == 0
      && length(google_cloud_run_v2_job.origin_postrestore) == 0
      && length(google_storage_bucket.private_files) == 0
      && length(google_service_account.ciphertext_uploader) == 0
    )
    error_message = "No origin rehearsal resource may be planned while Foundation is off."
  }
}

run "postrestore_job_is_iam_only_generation_bound_and_read_only" {
  command = plan

  variables {
    run_id                             = "g03o-1017"
    enable_foundation                  = true
    confirm_origin_rehearsal           = true
    enable_origin_postrestore_job      = true
    confirm_origin_bootstrap_completed = true
    bootstrap_evidence_sha256          = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_origin_import_completed    = true
    origin_import_evidence_sha256      = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    import_receipt_object              = "runs/g03o-1017/receipt.json"
    import_receipt_generation          = "9"
    import_receipt_bytes               = 2048
    import_receipt_sha256              = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    import_manifest_sha256             = "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30"
    import_toc_object                  = "runs/g03o-1017/toc.list"
    import_toc_generation              = "10"
    import_toc_bytes                   = 8192
    import_toc_sha256                  = "0677025fc73b036b9da9804ec457a8beea737662c9cf8537ba1bddd377b9b942"
    import_normalization_object        = "runs/g03o-1017/normalization.sql"
    import_normalization_generation    = "11"
    import_normalization_bytes         = 16384
    import_normalization_sha256        = "865697a41f9ca70881522f7558093e8ac4107dfa5a4e58dbf749d5f670eff842"
    import_migration_sha256            = "eca6b4295b593bc3ea3af470fb747bd6e788e141df4a57c8a0fae91ea6ad1fb2"
    origin_import_image                = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-import@sha256:2222222222222222222222222222222222222222222222222222222222222222"
    origin_postrestore_image           = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-import@sha256:2222222222222222222222222222222222222222222222222222222222222222"
    origin_proxy_image                 = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:4444444444444444444444444444444444444444444444444444444444444444"
  }

  assert {
    condition = (length(google_cloud_run_v2_job.origin_postrestore) == 1 &&
      google_cloud_run_v2_job.origin_postrestore[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.origin_postrestore[0].template[0].parallelism == 1 &&
      google_cloud_run_v2_job.origin_postrestore[0].template[0].template[0].max_retries == 0 &&
    length(google_cloud_run_v2_job.origin_postrestore[0].template[0].template[0].containers) == 2)
    error_message = "Post-restore verification must run once with no automatic retries and one proxy sidecar."
  }
  assert {
    condition = (contains(google_cloud_run_v2_job.origin_postrestore[0].template[0].template[0].containers[0].depends_on, "cloud-sql-proxy") &&
      contains(google_cloud_run_v2_job.origin_postrestore[0].template[0].template[0].containers[1].args, "--auto-iam-authn") &&
    contains(google_cloud_run_v2_job.origin_postrestore[0].template[0].template[0].containers[1].args, "--quitquitquit"))
    error_message = "Verifier must wait for the automatic IAM proxy and request bounded shutdown."
  }
  assert {
    condition     = google_sql_user.origin_postrestore[0].database_roles == tolist(["espacigo_gcp03_observer"])
    error_message = "Verifier SQL user must be assigned only to the existing read-only observer group."
  }
  assert {
    condition = (google_storage_bucket_iam_member.origin_postrestore_artifact_viewer[0].role == "roles/storage.objectViewer" &&
      strcontains(google_storage_bucket_iam_member.origin_postrestore_artifact_viewer[0].condition[0].expression, "objects/runs/g03o-1017/toc.list") &&
      strcontains(google_storage_bucket_iam_member.origin_postrestore_artifact_viewer[0].condition[0].expression, "objects/runs/g03o-1017/normalization.sql") &&
    length(google_storage_bucket_iam_member.private_files_creator) == 0)
    error_message = "Verifier must have access only to exact object reads."
  }
  assert {
    condition     = length(google_cloud_run_v2_job.origin_import) == 0
    error_message = "Post-restore verification test must not enable the import job."
  }
}

run "ciphertext_uploader_has_only_create_prefix_access" {
  command = plan

  variables {
    run_id                            = "g03o-1017"
    enable_foundation                 = true
    confirm_origin_rehearsal          = true
    enable_origin_ciphertext_uploader = true
  }

  assert {
    condition     = length(google_service_account.ciphertext_uploader) == 1 && google_service_account.ciphertext_uploader[0].account_id == "g03o-upload-g03o-1017"
    error_message = "Uploader must use a separate run-scoped service account."
  }
  assert {
    condition     = length(google_storage_bucket_iam_member.ciphertext_uploader) == 1 && google_storage_bucket_iam_member.ciphertext_uploader[0].role == "roles/storage.objectCreator" && strcontains(google_storage_bucket_iam_member.ciphertext_uploader[0].condition[0].expression, "objects/runs/g03o-1017/")
    error_message = "Uploader must have create-only access to only the run prefix."
  }
  assert {
    condition     = length(google_project_iam_member.admin_sql_client) == 1 && length(google_secret_manager_secret_iam_member.transport_key_reader) == 0
    error_message = "Uploader must receive no Cloud SQL or K_run secret access."
  }
}

run "import_job_is_single_attempt_private_and_hash_bound" {
  command = plan

  variables {
    run_id                                = "g03o-1017"
    enable_foundation                     = true
    confirm_origin_rehearsal              = true
    enable_origin_import_job              = true
    confirm_origin_bootstrap_completed    = true
    bootstrap_evidence_sha256             = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    admin_password_secret_version         = "3"
    confirm_admin_password_secret_version = true
    transport_key_secret_version          = "7"
    confirm_transport_key_secret_version  = true
    origin_executor_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    origin_import_image                   = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-import@sha256:2222222222222222222222222222222222222222222222222222222222222222"
    origin_proxy_image                    = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    import_receipt_object                 = "runs/g03o-1017/receipt.json"
    import_receipt_generation             = "9"
    import_receipt_bytes                  = 2048
    import_receipt_sha256                 = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    import_manifest_sha256                = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    import_observer_sha256                = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
    import_normalization_sha256           = "1111111111111111111111111111111111111111111111111111111111111111"
  }

  assert {
    condition     = length(google_cloud_run_v2_job.origin_import) == 1 && google_cloud_run_v2_job.origin_import[0].template[0].task_count == 1 && google_cloud_run_v2_job.origin_import[0].template[0].parallelism == 1 && google_cloud_run_v2_job.origin_import[0].template[0].template[0].max_retries == 0
    error_message = "Import must be a single-task, zero-retry Job."
  }
  assert {
    condition     = length(google_cloud_run_v2_job.origin_import[0].template[0].template[0].containers) == 2 && contains(google_cloud_run_v2_job.origin_import[0].template[0].template[0].containers[0].depends_on, "cloud-sql-proxy")
    error_message = "Import container must wait for the Cloud SQL Proxy sidecar."
  }
  assert {
    condition = (
      contains(google_cloud_run_v2_job.origin_import[0].template[0].template[0].containers[1].args, "--quitquitquit") &&
      contains(google_cloud_run_v2_job.origin_import[0].template[0].template[0].containers[1].args, "--exit-zero-on-sigterm") &&
      google_cloud_run_v2_job.origin_import[0].template[0].template[0].containers[1].startup_probe[0].http_get[0].path == "/startup" &&
      google_cloud_run_v2_job.origin_import[0].template[0].template[0].containers[1].startup_probe[0].http_get[0].port == 9801
    )
    error_message = "Proxy sidecar must expose controlled shutdown and a readiness startup probe."
  }
  assert {
    condition     = length(google_cloud_run_v2_job.origin_import[0].template[0].template[0].volumes) == 2
    error_message = "Import must mount exactly the admin password and K_run secret versions."
  }
  assert {
    condition     = google_storage_bucket.private_files[0].public_access_prevention == "enforced" && length(google_storage_bucket_iam_member.private_files_creator) == 1 && google_storage_bucket_iam_member.private_files_creator[0].role == "roles/storage.objectCreator"
    error_message = "Private files must use an enforced private bucket and create-only writer."
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
  # Use the mock provider's apply phase so its computed service-account email
  # is known and the IAM database-login relationship can be asserted.
  command = apply

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
      google_sql_user.origin_observer[0].name == trimsuffix(google_service_account.origin_observer[0].email, ".gserviceaccount.com") &&
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
