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
      length(google_service_account_iam_member.bootstrap_deployer_act_as) == 0 &&
      length(google_cloud_run_v2_job_iam_member.bootstrap_invoker) == 0 &&
      length(google_service_account_iam_member.restore_operator_act_as) == 0 &&
      length(google_service_account_iam_member.verify_operator_act_as) == 0 &&
      length(google_cloud_run_v2_job_iam_member.restore_invoker) == 0 &&
      length(google_cloud_run_v2_job_iam_member.verify_invoker) == 0 &&
      length(google_cloud_run_v2_job_iam_member.retire_invoker) == 0
    )
    error_message = "The managed synthetic restore must create no resources unless all exact review inputs are supplied."
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
