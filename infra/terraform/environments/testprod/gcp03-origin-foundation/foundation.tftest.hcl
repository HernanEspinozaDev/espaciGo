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
      length(google_cloud_run_v2_job.origin_membership) == 0 &&
      length(google_cloud_run_v2_job.origin_membership_diagnostic) == 0 &&
      length(google_cloud_run_v2_job.origin_owner_acl_diagnostic) == 0 &&
      length(google_cloud_run_v2_job.origin_observer) == 0 &&
      length(google_service_account.origin_observer) == 0 &&
      length(google_sql_user.origin_observer) == 0
      && length(google_cloud_run_v2_job.origin_import) == 0
      && length(google_cloud_run_v2_job.origin_postrestore) == 0
      && length(google_cloud_run_v2_job.origin_recovery) == 0
      && length(google_storage_bucket.private_files) == 0
      && length(google_service_account.ciphertext_uploader) == 0
      && length(google_project_iam_custom_role.uploader_token_access) == 0
      && length(google_service_account_iam_member.uploader_token_access) == 0
    )
    error_message = "No origin rehearsal resource may be planned while Foundation is off."
  }
}

run "recovery_plan_job_is_default_off_and_read_only_profile" {
  command = plan
  variables {
    run_id                                = "g03o-1017"
    enable_foundation                     = true
    confirm_origin_rehearsal              = true
    enable_origin_import_infrastructure   = true
    enable_origin_recovery_plan_job       = true
    confirm_origin_bootstrap_completed    = true
    confirm_origin_import_completed       = true
    origin_import_evidence_sha256         = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    bootstrap_evidence_sha256             = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version         = "1"
    confirm_admin_password_secret_version = true
    transport_key_secret_version          = "1"
    confirm_transport_key_secret_version  = true
    origin_recovery_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-recovery@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    origin_proxy_image                    = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    import_receipt_object                 = "runs/g03o-1017/receipt.json"
    import_receipt_generation             = "1792273080133875"
    import_receipt_bytes                  = 1892
    import_receipt_sha256                 = "42f817e89382af2df3e0f452b594311936e91839fd92cc251f133bd4d867377c"
    import_manifest_sha256                = "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30"
    import_observer_sha256                = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    import_normalization_sha256           = "2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2"
  }
  assert {
    condition = (length(google_cloud_run_v2_job.origin_recovery) == 1 &&
      google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].timeout == "1800s" &&
      google_cloud_run_v2_job.origin_recovery[0].template[0].task_count == 1 &&
      one([for e in google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_ORIGIN_RECOVERY_MODE"]) == "plan" &&
      one([for e in google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_ORIGIN_RECOVERY_DIAGNOSTIC_SHA256"]) == "3325288140b1f2de8bb944f7c4c1599fcf56f1a19531dc334bf171629f99391a" &&
    one([for v in google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].volumes : v.secret[0].items[0].version if v.name == "origin-transport-key"]) == "1")
    error_message = "Recovery planning must be one-task, zero-retry, plan-only and use the exact existing K_run version."
  }
  assert {
    condition     = length(google_cloud_run_v2_job.origin_import) == 0 && length(google_cloud_run_v2_job.origin_postrestore) == 0
    error_message = "Recovery planning must not create or execute the importer or verifier Job."
  }
}

run "recovery_apply_requires_explicit_plan_review_and_exact_hash" {
  command = plan
  variables {
    run_id                                = "g03o-1017"
    enable_foundation                     = true
    confirm_origin_rehearsal              = true
    enable_origin_import_infrastructure   = true
    enable_origin_recovery_plan_job       = true
    enable_origin_recovery_apply_job      = true
    confirm_origin_recovery_plan_reviewed = true
    origin_recovery_approved_plan_sha256  = "1111111111111111111111111111111111111111111111111111111111111111"
    confirm_origin_bootstrap_completed    = true
    confirm_origin_import_completed       = true
    origin_import_evidence_sha256         = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    bootstrap_evidence_sha256             = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version         = "1"
    confirm_admin_password_secret_version = true
    transport_key_secret_version          = "1"
    confirm_transport_key_secret_version  = true
    origin_recovery_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-recovery@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    origin_proxy_image                    = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    import_receipt_object                 = "runs/g03o-1017/receipt.json"
    import_receipt_generation             = "1792273080133875"
    import_receipt_bytes                  = 1892
    import_receipt_sha256                 = "42f817e89382af2df3e0f452b594311936e91839fd92cc251f133bd4d867377c"
    import_manifest_sha256                = "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30"
    import_observer_sha256                = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    import_normalization_sha256           = "2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2"
  }
  assert {
    condition = (one([for e in google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_ORIGIN_RECOVERY_MODE"]) == "apply" &&
      google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].timeout == "14400s" &&
    one([for e in google_cloud_run_v2_job.origin_recovery[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_ORIGIN_RECOVERY_APPROVED_PLAN_SHA256"]) == "1111111111111111111111111111111111111111111111111111111111111111")
    error_message = "Apply mode must be separately enabled and carry the reviewed plan SHA-256."
  }
}

run "runtime_membership_job_is_one_shot_and_separate_from_bootstrap_import" {
  command = plan
  variables {
    run_id                                = "g03o-1017"
    enable_foundation                     = true
    confirm_origin_rehearsal              = true
    enable_origin_membership_job          = true
    origin_membership_image               = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    admin_password_secret_version         = "3"
    confirm_admin_password_secret_version = true
  }
  assert {
    condition = (length(google_cloud_run_v2_job.origin_membership) == 1 &&
      google_cloud_run_v2_job.origin_membership[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.origin_membership[0].template[0].task_count == 1 &&
    google_cloud_run_v2_job.origin_membership[0].template[0].parallelism == 1)
    error_message = "Membership remediation must be a single no-retry task."
  }
  assert {
    condition = (length(google_cloud_run_v2_job.origin_bootstrap) == 0 &&
      length(google_cloud_run_v2_job.origin_import) == 0 &&
    length(google_cloud_run_v2_job.origin_postrestore) == 0)
    error_message = "Membership remediation must not rerun bootstrap, import, or post-restore."
  }
}

run "membership_diagnostic_job_is_read_only_profile_one_shot_and_separate" {
  command = apply
  variables {
    run_id                                  = "g03o-1017"
    enable_foundation                       = true
    confirm_origin_rehearsal                = true
    enable_origin_bootstrap_job             = true
    origin_executor_image                   = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    enable_origin_membership_job            = true
    origin_membership_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    enable_origin_membership_diagnostic_job = true
    origin_membership_diagnostic_image      = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    admin_password_secret_version           = "1"
    confirm_admin_password_secret_version   = true
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.origin_membership_diagnostic) == 1 &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].name == "espacigo-gcp03-origin-g03o-1017-runtime-membership-diagnostic" &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].parallelism == 1 &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].timeout == "900s" &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].service_account == google_service_account.admin[0].email &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].containers[0].image == var.origin_membership_diagnostic_image &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].containers[0].resources[0].limits["cpu"] == "1" &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].containers[0].resources[0].limits["memory"] == "1Gi" &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].volumes[0].secret[0].items[0].version == var.admin_password_secret_version &&
      google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].vpc_access[0].egress == "PRIVATE_RANGES_ONLY" &&
      one([for e in google_cloud_run_v2_job.origin_membership_diagnostic[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_ORIGIN_PHASE"]) == "membership-check"
    )
    error_message = "Membership diagnosis must be a fixed, private, one-task, zero-retry read-only phase with the reviewed image and admin secret identity."
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.origin_membership) == 1 &&
      length(google_cloud_run_v2_job.origin_import) == 0 &&
      length(google_cloud_run_v2_job.origin_postrestore) == 0
    )
    error_message = "The existing membership Job gate must remain enabled while import and restore Jobs remain disabled."
  }
}

run "uploader_impersonation_role_has_only_access_token_permission" {
  command = plan
  variables {
    run_id                                = "g03o-1017"
    enable_foundation                     = true
    confirm_origin_rehearsal              = true
    enable_origin_ciphertext_uploader     = true
    enable_uploader_impersonation_binding = true
  }
  assert {
    condition = (length(google_project_iam_custom_role.uploader_token_access) == 1 &&
    google_project_iam_custom_role.uploader_token_access[0].permissions == toset(["iam.serviceAccounts.getAccessToken"]))
    error_message = "The temporary custom role must contain exactly getAccessToken."
  }
  assert {
    condition = (length(google_service_account_iam_member.uploader_token_access) == 1 &&
    google_service_account_iam_member.uploader_token_access[0].member == "user:pasteleriahijitos@gmail.com")
    error_message = "The human binding must target only the exact run uploader service account."
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
    import_normalization_bytes         = 13047
    import_normalization_sha256        = "2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2"
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
    enable_origin_import_infrastructure   = true
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

run "owner_acl_diagnostic_adds_one_readonly_job_and_preserves_import_job" {
  command = plan

  variables {
    run_id                                 = "g03o-1017"
    enable_foundation                      = true
    confirm_origin_rehearsal               = true
    enable_origin_import_infrastructure    = true
    enable_origin_import_job               = true
    enable_origin_owner_acl_diagnostic_job = true
    confirm_origin_bootstrap_completed     = true
    bootstrap_evidence_sha256              = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    admin_password_secret_version          = "1"
    confirm_admin_password_secret_version  = true
    transport_key_secret_version           = "7"
    confirm_transport_key_secret_version   = true
    origin_executor_image                  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    origin_owner_acl_diagnostic_image      = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    origin_import_image                    = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-import@sha256:2222222222222222222222222222222222222222222222222222222222222222"
    origin_proxy_image                     = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    import_receipt_object                  = "runs/g03o-1017/receipt.json"
    import_receipt_generation              = "9"
    import_receipt_bytes                   = 2048
    import_receipt_sha256                  = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    import_manifest_sha256                 = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    import_observer_sha256                 = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
    import_normalization_sha256            = "1111111111111111111111111111111111111111111111111111111111111111"
  }

  assert {
    condition     = length(google_cloud_run_v2_job.origin_owner_acl_diagnostic) == 1 && length(google_cloud_run_v2_job.origin_import) == 1
    error_message = "Enabling the diagnostic must preserve the existing import Job and add exactly one separate diagnostic Job."
  }
  assert {
    condition = (
      google_cloud_run_v2_job.origin_owner_acl_diagnostic[0].template[0].template[0].max_retries == 0 &&
      contains([for env in google_cloud_run_v2_job.origin_owner_acl_diagnostic[0].template[0].template[0].containers[0].env : env.value if env.name == "GCP03_ORIGIN_PHASE"], "owner-acl-check") &&
      one([for volume in google_cloud_run_v2_job.origin_owner_acl_diagnostic[0].template[0].template[0].volumes : volume.secret[0].items[0].version if volume.name == "origin-admin-password"]) == "1"
    )
    error_message = "The diagnostic must use its fixed read-only profile, one attempt, and admin secret version 1."
  }
}

run "import_infrastructure_gate_does_not_create_import_job" {
  command = plan

  variables {
    run_id                              = "g03o-1017"
    enable_foundation                   = true
    confirm_origin_rehearsal            = true
    enable_origin_import_infrastructure = true
    enable_origin_import_job            = false
    enable_origin_postrestore_job       = false
  }

  assert {
    condition     = length(google_storage_bucket.private_files) == 1 && length(google_secret_manager_secret.transport_key) == 1 && length(google_secret_manager_secret_iam_member.transport_key_reader) == 1
    error_message = "Import infrastructure gate must create only the private bucket and transport-key secret metadata/access."
  }
  assert {
    condition     = length(google_cloud_run_v2_job.origin_import) == 0 && length(google_cloud_run_v2_job.origin_postrestore) == 0
    error_message = "Import infrastructure gate must leave import and post-restore Jobs absent."
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
