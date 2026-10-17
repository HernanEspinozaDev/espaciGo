mock_provider "google" {
  override_during = plan
  mock_data "google_compute_network" {
    defaults = { self_link = "projects/espacigo-testprod/global/networks/espacigo-testprod" }
  }
  mock_data "google_compute_subnetwork" {
    defaults = { private_ip_google_access = true, region = "southamerica-west1" }
  }
}

run "restore_is_off_by_default" {
  command = plan
  variables {
    project_id = "espacigo-testprod"
    region     = "southamerica-west1"
  }
  assert {
    condition = (
      length(google_sql_database_instance.restore) == 0 &&
      length(google_sql_database.restore) == 0 &&
      length(google_storage_bucket.artifacts) == 0 &&
      length(google_secret_manager_secret.postgres_password) == 0 &&
      length(google_cloud_run_v2_job.restore) == 0 &&
      length(google_cloud_run_v2_job.verify) == 0 &&
      length(google_cloud_run_v2_job.admin_phase) == 0 &&
      length(google_cloud_run_v2_job.diagnose) == 0 &&
      length(google_cloud_run_v2_job.retirement_diagnose) == 0 &&
      length(google_storage_bucket_iam_member.diagnostic_artifact_reader) == 0 &&
      length(google_service_account_iam_member.bootstrap_deployer_act_as) == 0 &&
      length(google_cloud_run_v2_job_iam_member.bootstrap_invoker) == 0 &&
      length(google_service_account_iam_member.restore_operator_act_as) == 0 &&
      length(google_service_account_iam_member.verify_operator_act_as) == 0 &&
      length(google_cloud_run_v2_job_iam_member.restore_invoker) == 0 &&
      length(google_cloud_run_v2_job_iam_member.verify_invoker) == 0 &&
      length(google_cloud_run_v2_job_iam_member.retire_invoker) == 0
      && length(google_cloud_run_v2_job_iam_member.diagnostic_invoker) == 0
      && length(google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker) == 0
    )
    error_message = "The managed synthetic restore must create no resources unless all exact review inputs are supplied."
  }
}

run "retirement_diagnosis_is_separately_gated_and_uses_existing_admin_identity" {
  command = plan
  variables {
    project_id                       = "espacigo-testprod"
    region                           = "southamerica-west1"
    run_id                           = "fixture1"
    enable_foundation                = true
    enable_bootstrap_job             = true
    enable_restore_jobs              = true
    enable_retirement_diagnostic_job = true
    confirm_bootstrap_complete       = true
    confirm_isolated_restore         = true
    executor_image                   = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    retirement_diagnostic_image      = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03restore@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    proxy_image                      = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version    = "7"
    manifest_sha256                  = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    migration_inventory_sha256       = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    bootstrap_operator_member        = "user:pasteleriahijitos@gmail.com"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.retirement_diagnose) == 1 &&
      google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].service_account == local.admin_service_account_email &&
      google_cloud_run_v2_job.retirement_diagnose[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].timeout == "300s" &&
      one([for c in google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].containers : c.image if c.name == "retire-diagnose"]) == var.retirement_diagnostic_image &&
      one([for e in google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_PHASE"]) == "retire-diagnose" &&
      one([for e in google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].containers[0].env : e.value if e.name == "GCP03_ADMIN_PASSWORD_FILE"]) == "/secrets/postgres/password" &&
      one([for e in google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].containers[0].env : e.value if e.name == "DATABASE_URL"]) == "postgres://postgres@127.0.0.1:5432/espacigo_scratch?sslmode=disable" &&
      one([for v in google_cloud_run_v2_job.retirement_diagnose[0].template[0].template[0].volumes : v.secret[0].secret if v.name == "admin-password"]) == "espacigo-gcp03-restore-fixture1-postgres-password"
    )
    error_message = "Retirement diagnosis must be separately gated, single-task, no-retry, fixed-target, and use only the already-authorized run-scoped admin identity and secret version."
  }
  assert {
    condition = (
      length(google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker) == 1 &&
      google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker[0].name == google_cloud_run_v2_job.retirement_diagnose[0].name &&
      google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker[0].role == "roles/run.invoker" &&
      google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker[0].member == var.bootstrap_operator_member &&
      length(google_sql_user.restore_iam) == 1 && length(google_sql_user.verify_iam) == 1
    )
    error_message = "The diagnosis Job receives only its run-scoped invocation grant and does not alter existing Cloud SQL user identities."
  }
}

run "diagnostic_job_is_read_only_and_scoped" {
  command = plan
  variables {
    project_id                    = "espacigo-testprod"
    region                        = "southamerica-west1"
    run_id                        = "fixture1"
    enable_foundation             = true
    enable_bootstrap_job          = true
    enable_restore_jobs           = true
    enable_diagnostic_job         = true
    confirm_bootstrap_complete    = true
    confirm_isolated_restore      = true
    executor_image                = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    diagnostic_image              = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03restore@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    proxy_image                   = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version = "7"
    manifest_sha256               = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    migration_inventory_sha256    = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    bootstrap_operator_member     = "user:pasteleriahijitos@gmail.com"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.diagnose) == 1 &&
      google_cloud_run_v2_job.diagnose[0].template[0].template[0].service_account == local.verify_service_account_email &&
      google_cloud_run_v2_job.diagnose[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.diagnose[0].template[0].template[0].timeout == "300s" &&
      one([for container in google_cloud_run_v2_job.diagnose[0].template[0].template[0].containers : container.image if container.name == "diagnose"]) == var.diagnostic_image &&
      one([for env in google_cloud_run_v2_job.diagnose[0].template[0].template[0].containers[0].env : env.value if env.name == "GCP03_PHASE"]) == "diagnose" &&
      one([for env in google_cloud_run_v2_job.diagnose[0].template[0].template[0].containers[0].env : env.value if env.name == "DATABASE_URL"]) == "postgres://g03-verify-fixture1%40espacigo-testprod.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable" &&
      length([for env in google_cloud_run_v2_job.diagnose[0].template[0].template[0].containers[0].env : env if env.name == "GCP03_ADMIN_PASSWORD_FILE"]) == 0
    )
    error_message = "Diagnostics must use the existing verifier identity, bounded execution, no retry, no secret volume, and the fixed IAM SQL URL."
  }
  assert {
    condition = (
      length(google_storage_bucket_iam_member.diagnostic_artifact_reader) == 1 &&
      google_storage_bucket_iam_member.diagnostic_artifact_reader[0].role == "roles/storage.objectViewer" &&
      length(google_cloud_run_v2_job_iam_member.diagnostic_invoker) == 1 &&
      google_cloud_run_v2_job_iam_member.diagnostic_invoker[0].name == google_cloud_run_v2_job.diagnose[0].name &&
      google_cloud_run_v2_job_iam_member.diagnostic_invoker[0].role == "roles/run.invoker" &&
      google_cloud_run_v2_job_iam_member.diagnostic_invoker[0].member == "user:pasteleriahijitos@gmail.com" &&
      google_cloud_run_v2_job.restore[0].template[0].template[0].containers[0].image == var.executor_image
    )
    error_message = "The diagnostic receives only objectViewer on the run bucket and exact Job invocation; the existing restore image remains unchanged until its own recovery review."
  }
}

run "iam_user_closeout_gate_does_not_lower_shared_protections" {
  command = plan
  variables {
    project_id                     = "espacigo-testprod"
    region                         = "southamerica-west1"
    run_id                         = "fixture1"
    enable_foundation              = true
    enable_bootstrap_job           = true
    enable_restore_jobs            = true
    confirm_bootstrap_complete     = true
    confirm_isolated_restore       = true
    allow_restore_iam_user_destroy = true
    confirm_destroy_run_id         = "fixture1"
    admin_password_secret_version  = "7"
    executor_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    proxy_image                    = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    manifest_sha256                = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    migration_inventory_sha256     = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
  }

  assert {
    condition = (
      google_sql_user.restore_iam[0].deletion_policy == "DELETE" &&
      google_sql_user.verify_iam[0].deletion_policy == "DELETE" &&
      google_sql_database_instance.restore[0].deletion_policy == "PREVENT" &&
      google_sql_database_instance.restore[0].deletion_protection &&
      google_sql_database_instance.restore[0].settings[0].deletion_protection_enabled &&
      google_cloud_run_v2_job.restore[0].deletion_protection &&
      google_cloud_run_v2_job.verify[0].deletion_protection
    )
    error_message = "The run-bound IAM-user gate must permit only the two login deletions without lowering SQL instance or Job protections."
  }
}

run "restore_image_override_requires_diagnostic_baseline" {
  command = plan
  variables {
    project_id                    = "espacigo-testprod"
    region                        = "southamerica-west1"
    run_id                        = "fixture1"
    enable_foundation             = true
    enable_bootstrap_job          = true
    enable_restore_jobs           = true
    confirm_bootstrap_complete    = true
    confirm_isolated_restore      = true
    executor_image                = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    restore_executor_image        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03restore@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    proxy_image                   = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version = "7"
    manifest_sha256               = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    migration_inventory_sha256    = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    bootstrap_operator_member     = "user:pasteleriahijitos@gmail.com"
  }
  expect_failures = [var.restore_executor_image]
}

run "reviewed_restore_override_changes_only_restore_job_image" {
  command = plan
  variables {
    project_id                       = "espacigo-testprod"
    region                           = "southamerica-west1"
    run_id                           = "fixture1"
    enable_foundation                = true
    enable_bootstrap_job             = true
    enable_restore_jobs              = true
    confirm_bootstrap_complete       = true
    confirm_isolated_restore         = true
    confirm_read_only_empty_baseline = true
    executor_image                   = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    restore_executor_image           = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03restore@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    diagnostic_evidence_sha256       = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
    proxy_image                      = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version    = "7"
    manifest_sha256                  = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    migration_inventory_sha256       = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    bootstrap_operator_member        = "user:pasteleriahijitos@gmail.com"
  }
  assert {
    condition = (
      google_cloud_run_v2_job.restore[0].template[0].template[0].containers[0].image == var.restore_executor_image &&
      google_cloud_run_v2_job.verify[0].template[0].template[0].containers[0].image == var.executor_image &&
      google_cloud_run_v2_job.admin_phase["bootstrap"].template[0].template[0].containers[0].image == var.executor_image &&
      google_cloud_run_v2_job.admin_phase["retire"].template[0].template[0].containers[0].image == var.executor_image
    )
    error_message = "The separately acknowledged recovery override may update only the restore Job image."
  }
}

run "bootstrap_only_plan_has_exact_operator_grants" {
  command = plan
  variables {
    project_id                    = "espacigo-testprod"
    region                        = "southamerica-west1"
    run_id                        = "fixture1"
    enable_foundation             = true
    enable_bootstrap_job          = true
    enable_restore_jobs           = false
    confirm_isolated_restore      = true
    executor_image                = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    proxy_image                   = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    admin_password_secret_version = "7"
    bootstrap_operator_member     = "user:pasteleriahijitos@gmail.com"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.admin_phase) == 1 &&
      contains(keys(google_cloud_run_v2_job.admin_phase), "bootstrap") &&
      length(google_cloud_run_v2_job.restore) == 0 &&
      length(google_cloud_run_v2_job.verify) == 0 &&
      length(google_sql_user.restore_iam) == 0 &&
      length(google_sql_user.verify_iam) == 0 &&
      length(google_service_account_iam_member.restore_operator_act_as) == 0 &&
      length(google_service_account_iam_member.verify_operator_act_as) == 0 &&
      length(google_cloud_run_v2_job_iam_member.restore_invoker) == 0 &&
      length(google_cloud_run_v2_job_iam_member.verify_invoker) == 0 &&
      length(google_cloud_run_v2_job_iam_member.retire_invoker) == 0
    )
    error_message = "The bootstrap-only gate must contain only the protected bootstrap Job and no restore, verify, retire, or IAM SQL users."
  }

  assert {
    condition = (
      length(google_service_account_iam_member.bootstrap_deployer_act_as) == 1 &&
      google_service_account_iam_member.bootstrap_deployer_act_as[0].role == "roles/iam.serviceAccountUser" &&
      google_service_account_iam_member.bootstrap_deployer_act_as[0].member == "user:pasteleriahijitos@gmail.com" &&
      length(google_cloud_run_v2_job_iam_member.bootstrap_invoker) == 1 &&
      google_cloud_run_v2_job_iam_member.bootstrap_invoker[0].name == google_cloud_run_v2_job.admin_phase["bootstrap"].name &&
      google_cloud_run_v2_job_iam_member.bootstrap_invoker[0].role == "roles/run.invoker" &&
      google_cloud_run_v2_job_iam_member.bootstrap_invoker[0].member == "user:pasteleriahijitos@gmail.com"
    )
    error_message = "Bootstrap invocation and actAs must be granted only to the authorized operator on the exact Job and admin service account."
  }
}

run "restore_resources_are_private_protected_and_single_shot" {
  command = plan
  variables {
    project_id                    = "espacigo-testprod"
    region                        = "southamerica-west1"
    run_id                        = "fixture1"
    enable_foundation             = true
    enable_bootstrap_job          = true
    enable_restore_jobs           = true
    confirm_bootstrap_complete    = true
    confirm_isolated_restore      = true
    executor_image                = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-restore-fixture1/restore@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    proxy_image                   = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    manifest_sha256               = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    migration_inventory_sha256    = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    admin_password_secret_version = "1"
  }

  assert {
    condition = (
      length(google_sql_database_instance.restore) == 1 &&
      length(google_sql_database_instance.restore[0].settings[0].database_flags) == 1 &&
      google_sql_database_instance.restore[0].name == "espacigo-gcp03-restore-fixture1-pg" &&
      google_sql_database_instance.restore[0].settings[0].ip_configuration[0].ipv4_enabled == false &&
      google_sql_database_instance.restore[0].settings[0].connector_enforcement == "REQUIRED" &&
      contains([for flag in google_sql_database_instance.restore[0].settings[0].database_flags : "${flag.name}=${flag.value}"], "cloudsql.iam_authentication=on")
    )
    error_message = "The restore SQL instance must be a separate private-IP target with connector enforcement."
  }
  assert {
    condition = (
      length(google_cloud_run_v2_job.restore) == 1 &&
      google_storage_bucket.artifacts[0].name == "espacigo-gcp03-restore-fixture1" &&
      length(google_cloud_run_v2_job.verify) == 1 &&
      length(google_cloud_run_v2_job.admin_phase) == 2 &&
      google_cloud_run_v2_job.restore[0].deletion_protection &&
      google_cloud_run_v2_job.restore[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.restore[0].template[0].parallelism == 1 &&
      google_cloud_run_v2_job.restore[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.restore[0].template[0].template[0].containers[0].image == var.executor_image &&
      google_cloud_run_v2_job.restore[0].template[0].template[0].containers[1].image == var.proxy_image &&
      contains(google_cloud_run_v2_job.restore[0].template[0].template[0].containers[1].args, "--admin-port=9091") &&
      contains(google_cloud_run_v2_job.restore[0].template[0].template[0].containers[1].args, "--quitquitquit") &&
      contains(google_cloud_run_v2_job.restore[0].template[0].template[0].containers[1].args, "--exit-zero-on-sigterm") &&
      one([for env in google_cloud_run_v2_job.restore[0].template[0].template[0].containers[0].env : env.value if env.name == "GCP03_BUCKET"]) == google_storage_bucket.artifacts[0].name
    )
    error_message = "Restore and proxy images must be digest-pinned in one protected, serial, zero-retry Job."
  }
  assert {
    condition = (
      length(google_sql_user.restore_iam[0].database_roles) == 1 &&
      google_sql_user.restore_iam[0].database_roles[0] == local.owner_role &&
      google_sql_user.restore_iam[0].name == "g03-restore-fixture1@espacigo-testprod.iam" &&
      length(google_sql_user.verify_iam[0].database_roles) == 1 &&
      google_sql_user.verify_iam[0].database_roles[0] == local.verifier_role &&
      google_sql_user.verify_iam[0].name == "g03-verify-fixture1@espacigo-testprod.iam"
    )
    error_message = "The restore and verification IAM SQL users must receive only their exact run-scoped database role."
  }
  assert {
    condition = (
      google_cloud_run_v2_job.verify[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.verify[0].template[0].template[0].service_account == local.verify_service_account_email &&
      contains(google_cloud_run_v2_job.verify[0].template[0].template[0].containers[1].args, "--quitquitquit") &&
      one([for env in google_cloud_run_v2_job.verify[0].template[0].template[0].containers[0].env : env.value if env.name == "DATABASE_URL"]) == "postgres://g03-verify-fixture1%40espacigo-testprod.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable" &&
      length([for env in google_cloud_run_v2_job.verify[0].template[0].template[0].containers[0].env : env if env.name == "GCP03_BUCKET"]) == 0
    )
    error_message = "Verification must be an independent zero-retry Job using the narrow verifier identity."
  }
  assert {
    condition = (
      google_cloud_run_v2_job.admin_phase["bootstrap"].template[0].template[0].service_account == local.admin_service_account_email &&
      google_cloud_run_v2_job.admin_phase["retire"].template[0].template[0].max_retries == 0 &&
      contains(google_cloud_run_v2_job.admin_phase["retire"].template[0].template[0].containers[1].args, "--quitquitquit") &&
      alltrue([for phase in ["bootstrap", "retire"] : one([for env in google_cloud_run_v2_job.admin_phase[phase].template[0].template[0].containers[0].env : env.value if env.name == "GCP03_PHASE"]) == phase]) &&
      alltrue([for phase in ["bootstrap", "retire"] : one([for env in google_cloud_run_v2_job.admin_phase[phase].template[0].template[0].containers[0].env : env.value if env.name == "GCP03_ADMIN_PASSWORD_FILE"]) == "/secrets/postgres/password"]) &&
      alltrue([for phase in ["bootstrap", "retire"] : length([for env in google_cloud_run_v2_job.admin_phase[phase].template[0].template[0].containers[0].env : env if env.name == "GCP03_BUCKET"]) == 0])
    )
    error_message = "Bootstrap and retirement must use the separate short-lived admin identity and no automatic retries."
  }
  assert {
    condition = (
      length(google_service_account_iam_member.restore_operator_act_as) == 1 &&
      google_service_account_iam_member.restore_operator_act_as[0].service_account_id == "projects/espacigo-testprod/serviceAccounts/g03-restore-fixture1@espacigo-testprod.iam.gserviceaccount.com" &&
      google_service_account_iam_member.restore_operator_act_as[0].role == "roles/iam.serviceAccountUser" &&
      google_service_account_iam_member.restore_operator_act_as[0].member == "user:pasteleriahijitos@gmail.com" &&
      length(google_service_account_iam_member.verify_operator_act_as) == 1 &&
      google_service_account_iam_member.verify_operator_act_as[0].service_account_id == "projects/espacigo-testprod/serviceAccounts/g03-verify-fixture1@espacigo-testprod.iam.gserviceaccount.com" &&
      google_service_account_iam_member.verify_operator_act_as[0].role == "roles/iam.serviceAccountUser" &&
      google_service_account_iam_member.verify_operator_act_as[0].member == "user:pasteleriahijitos@gmail.com" &&
      length(google_cloud_run_v2_job_iam_member.restore_invoker) == 1 &&
      google_cloud_run_v2_job_iam_member.restore_invoker[0].name == google_cloud_run_v2_job.restore[0].name &&
      google_cloud_run_v2_job_iam_member.restore_invoker[0].role == "roles/run.invoker" &&
      google_cloud_run_v2_job_iam_member.restore_invoker[0].member == "user:pasteleriahijitos@gmail.com" &&
      length(google_cloud_run_v2_job_iam_member.verify_invoker) == 1 &&
      google_cloud_run_v2_job_iam_member.verify_invoker[0].name == google_cloud_run_v2_job.verify[0].name &&
      google_cloud_run_v2_job_iam_member.verify_invoker[0].role == "roles/run.invoker" &&
      google_cloud_run_v2_job_iam_member.verify_invoker[0].member == "user:pasteleriahijitos@gmail.com" &&
      length(google_cloud_run_v2_job_iam_member.retire_invoker) == 1 &&
      google_cloud_run_v2_job_iam_member.retire_invoker[0].name == google_cloud_run_v2_job.admin_phase["retire"].name &&
      google_cloud_run_v2_job_iam_member.retire_invoker[0].role == "roles/run.invoker" &&
      google_cloud_run_v2_job_iam_member.retire_invoker[0].member == "user:pasteleriahijitos@gmail.com"
    )
    error_message = "Restore, verify, and retire invocation plus actAs on the restore and verify service accounts must be limited to the authorized operator and exact resources."
  }
}
