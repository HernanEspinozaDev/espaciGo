mock_provider "google" {
  override_during = plan

  mock_data "google_compute_network" {
    defaults = {
      self_link = "projects/espacigo-testprod/global/networks/espacigo-testprod"
    }
  }
  mock_data "google_compute_subnetwork" {
    defaults = {
      private_ip_google_access = true
      region                   = "southamerica-west1"
    }
  }
}

run "scratch_is_off_by_default" {
  command = plan

  variables {
    project_id = "espacigo-testprod"
    region     = "southamerica-west1"
  }

  assert {
    condition = (
      length(google_sql_database_instance.scratch) == 0 &&
      length(google_cloud_run_v2_job.migrator) == 0 &&
      length(google_cloud_run_v2_job.role_comment_probe) == 0 &&
      length(google_project_iam_custom_role.role_probe_cloudsql_operator) == 0 &&
      length(google_project_iam_custom_role.role_probe_secret_operator) == 0 &&
      length(google_project_iam_custom_role.replay_credential_sql_operator) == 0 &&
      length(google_project_iam_custom_role.replay_secret_version_operator) == 0
    )
    error_message = "Scratch resources, migration jobs, and replay credential IAM must remain disabled by default."
  }
}

run "job_creates_five_immutable_named_phases_with_exact_operator_iam" {
  command = plan

  variables {
    project_id                            = "espacigo-testprod"
    region                                = "southamerica-west1"
    run_id                                = "pilot01"
    enable_scratch                        = true
    enable_job                            = true
    confirm_project_inventory             = true
    confirm_image_pushed                  = true
    confirm_scratch_secret_version        = true
    confirm_database_owner_secret_version = true
    migrator_secret_version               = "4"
    database_owner_secret_version         = "1"
    migrator_image                        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                       = "user:operator@example.invalid"
  }

  assert {
    condition = toset(keys(google_cloud_run_v2_job.migrator)) == toset([
      "bootstrap", "database_acl", "migrate", "finalize", "retire"
    ])
    error_message = "The reviewed Job gate must create only the five named scratch phases."
  }

  assert {
    condition = alltrue([
      for phase, job in google_cloud_run_v2_job.migrator :
      job.name == "espacigo-gcp03-pilot01-${replace(phase, "_", "-")}" &&
      can(regex("^[a-z][a-z0-9-]{0,61}[a-z0-9]$", job.name)) &&
      job.deletion_protection &&
      job.template[0].task_count == 1 &&
      job.template[0].parallelism == 1 &&
      job.template[0].template[0].max_retries == 0 &&
      anytrue([for env in job.template[0].template[0].containers[0].env : env.name == "GCP03_PHASE" && env.value == phase])
    ])
    error_message = "Each named Job must have its matching phase and single-task no-retry execution settings."
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job_iam_member.invoke_operator) == 5 &&
      alltrue([for binding in values(google_cloud_run_v2_job_iam_member.invoke_operator) : binding.role == "roles/run.invoker" && binding.member == "user:operator@example.invalid"]) &&
      google_service_account_iam_member.operator_act_as[0].role == "roles/iam.serviceAccountUser" &&
      google_service_account_iam_member.operator_act_as_database_acl[0].role == "roles/iam.serviceAccountUser" &&
      google_project_iam_member.scratch_acl_cloudsql_client[0].role == "roles/cloudsql.client" &&
      google_service_account.scratch_database_acl[0].account_id == "gcp03-scratch-acl-pilot01"
    )
    error_message = "Only the named operator should receive invoker on each phase and actAs on the scratch service account."
  }

  assert {
    condition = alltrue([
      for phase, job in google_cloud_run_v2_job.migrator :
      job.template[0].template[0].service_account == (contains(["database_acl", "retire"], phase) ? local.scratch_database_acl_service_account_email : local.scratch_migrator_service_account_email) &&
      job.template[0].template[0].volumes[0].secret[0].secret == (contains(["database_acl", "retire"], phase) ? google_secret_manager_secret.scratch_database_owner_password[0].secret_id : google_secret_manager_secret.scratch_password[0].secret_id) &&
      job.template[0].template[0].volumes[0].secret[0].items[0].version == (contains(["database_acl", "retire"], phase) ? "1" : "4") &&
      anytrue([for env in job.template[0].template[0].containers[0].env : env.name == "DATABASE_PASSWORD_FILE" && env.value == (contains(["database_acl", "retire"], phase) ? "/secrets/database-owner/password" : "/secrets/migrator/password")])
    ])
    error_message = "ACL/retire must use only the owner identity and numeric owner secret; bootstrap/migrate/finalize must use only migrator identity and its numeric secret."
  }

  assert {
    condition = (
      length(google_project_iam_custom_role.replay_credential_sql_operator) == 0 &&
      length(google_project_iam_custom_role.replay_secret_version_operator) == 0 &&
      length(google_project_iam_member.replay_credential_sql_operator) == 0 &&
      length(google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator) == 0 &&
      length(google_secret_manager_secret_iam_member.replay_owner_secret_version_operator) == 0
    )
    error_message = "Enabling the separately gated Jobs must not activate replay credential IAM automatically."
  }
}

run "replay_credential_iam_coexists_with_five_jobs_and_preserves_exact_targets" {
  command = plan

  variables {
    project_id                               = "espacigo-testprod"
    region                                   = "southamerica-west1"
    run_id                                   = "pilot01"
    enable_scratch                           = true
    enable_job                               = true
    enable_replay_credential_permissions     = true
    confirm_project_inventory                = true
    confirm_project_scoped_cloudsql_user_ops = true
    replay_credential_iam_expires_at         = "2026-10-16T16:00:00Z"
    confirm_image_pushed                     = true
    confirm_scratch_secret_version           = true
    confirm_database_owner_secret_version    = true
    migrator_secret_version                  = "4"
    database_owner_secret_version            = "1"
    migrator_image                           = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                          = "user:pasteleriahijitos@gmail.com"
  }

  assert {
    condition = (
      toset(keys(google_cloud_run_v2_job.migrator)) == toset(["bootstrap", "database_acl", "migrate", "finalize", "retire"]) &&
      length(google_project_iam_custom_role.replay_credential_sql_operator) == 1 &&
      length(google_project_iam_member.replay_credential_sql_operator) == 1 &&
      length(google_project_iam_custom_role.replay_secret_version_operator) == 1 &&
      length(google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator) == 1 &&
      length(google_secret_manager_secret_iam_member.replay_owner_secret_version_operator) == 1
    )
    error_message = "The separate replay IAM state must remain configured alongside all five fixed Jobs."
  }

  assert {
    condition = (
      google_project_iam_custom_role.replay_credential_sql_operator[0].role_id == "gcp03v1v61sqlops261016" &&
      toset(google_project_iam_custom_role.replay_credential_sql_operator[0].permissions) == toset(["cloudsql.instances.get", "cloudsql.users.create", "cloudsql.users.get", "cloudsql.users.update"]) &&
      google_project_iam_member.replay_credential_sql_operator[0].member == "user:pasteleriahijitos@gmail.com" &&
      google_project_iam_member.replay_credential_sql_operator[0].condition[0].expression == "request.time < timestamp('2026-10-16T16:00:00Z')" &&
      google_project_iam_custom_role.replay_secret_version_operator[0].role_id == "gcp03v1v61secretversions261016" &&
      toset(google_project_iam_custom_role.replay_secret_version_operator[0].permissions) == toset(["secretmanager.versions.add", "secretmanager.versions.destroy", "secretmanager.versions.get", "secretmanager.versions.list"]) &&
      google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator[0].secret_id == "espacigo-gcp03-pilot01-migrator-password" &&
      google_secret_manager_secret_iam_member.replay_owner_secret_version_operator[0].secret_id == "espacigo-gcp03-pilot01-db-owner-password" &&
      google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator[0].member == "user:pasteleriahijitos@gmail.com" &&
      google_secret_manager_secret_iam_member.replay_owner_secret_version_operator[0].member == "user:pasteleriahijitos@gmail.com"
    )
    error_message = "Replay credential IAM must retain its separate custom IDs, limited permissions, operator, expiry, and two exact secret scopes while Jobs are enabled."
  }

  assert {
    condition = alltrue([
      for phase, job in google_cloud_run_v2_job.migrator :
      job.template[0].template[0].service_account == (contains(["database_acl", "retire"], phase) ? local.scratch_database_acl_service_account_email : local.scratch_migrator_service_account_email) &&
      job.template[0].template[0].volumes[0].secret[0].secret == (contains(["database_acl", "retire"], phase) ? google_secret_manager_secret.scratch_database_owner_password[0].secret_id : google_secret_manager_secret.scratch_password[0].secret_id) &&
      job.template[0].template[0].volumes[0].secret[0].items[0].version == (contains(["database_acl", "retire"], phase) ? "1" : "4") &&
      anytrue([for env in job.template[0].template[0].containers[0].env : env.name == "DATABASE_PASSWORD_FILE" && env.value == (contains(["database_acl", "retire"], phase) ? "/secrets/database-owner/password" : "/secrets/migrator/password")])
    ])
    error_message = "Five-phase Jobs must map ACL/retire to the owner service account/secret and bootstrap/migrate/finalize to the migrator service account/secret with numeric versions."
  }
}

run "sequential_mocked_replay_iam_seed" {
  command   = apply
  state_key = "replay-iam-then-jobs"

  variables {
    project_id                               = "espacigo-testprod"
    region                                   = "southamerica-west1"
    run_id                                   = "pilot01"
    enable_scratch                           = true
    enable_replay_credential_permissions     = true
    confirm_project_inventory                = true
    confirm_project_scoped_cloudsql_user_ops = true
    replay_credential_iam_expires_at         = "2026-10-16T16:00:00Z"
    operator_member                          = "user:pasteleriahijitos@gmail.com"
  }

  assert {
    condition = (
      length(google_project_iam_custom_role.replay_credential_sql_operator) == 1 &&
      length(google_project_iam_member.replay_credential_sql_operator) == 1 &&
      length(google_project_iam_custom_role.replay_secret_version_operator) == 1 &&
      length(google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator) == 1 &&
      length(google_secret_manager_secret_iam_member.replay_owner_secret_version_operator) == 1 &&
      length(google_cloud_run_v2_job.migrator) == 0
    )
    error_message = "The first isolated mock apply must create only the five replay IAM resources, with Jobs still disabled."
  }
}

run "sequential_jobs_plan_retains_applied_replay_iam" {
  command   = plan
  state_key = "replay-iam-then-jobs"

  variables {
    project_id                               = "espacigo-testprod"
    region                                   = "southamerica-west1"
    run_id                                   = "pilot01"
    enable_scratch                           = true
    enable_job                               = true
    enable_replay_credential_permissions     = true
    confirm_project_inventory                = true
    confirm_project_scoped_cloudsql_user_ops = true
    replay_credential_iam_expires_at         = "2026-10-16T16:00:00Z"
    confirm_image_pushed                     = true
    confirm_scratch_secret_version           = true
    confirm_database_owner_secret_version    = true
    migrator_secret_version                  = "4"
    database_owner_secret_version            = "1"
    migrator_image                           = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                          = "user:pasteleriahijitos@gmail.com"
  }

  assert {
    condition = (
      toset(keys(google_cloud_run_v2_job.migrator)) == toset(["bootstrap", "database_acl", "migrate", "finalize", "retire"]) &&
      length(google_project_iam_custom_role.replay_credential_sql_operator) == 1 &&
      length(google_project_iam_member.replay_credential_sql_operator) == 1 &&
      length(google_project_iam_custom_role.replay_secret_version_operator) == 1 &&
      length(google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator) == 1 &&
      length(google_secret_manager_secret_iam_member.replay_owner_secret_version_operator) == 1 &&
      google_project_iam_member.replay_credential_sql_operator[0].condition[0].expression == "request.time < timestamp('2026-10-16T16:00:00Z')" &&
      google_secret_manager_secret_iam_member.replay_migrator_secret_version_operator[0].secret_id == "espacigo-gcp03-pilot01-migrator-password" &&
      google_secret_manager_secret_iam_member.replay_owner_secret_version_operator[0].secret_id == "espacigo-gcp03-pilot01-db-owner-password"
    )
    error_message = "Planning the five Jobs after the isolated IAM apply must retain all IAM grants, their expiry, and exact secret scopes."
  }
}

run "replay_job_cleanup_seed" {
  command   = apply
  state_key = "replay-job-cleanup"

  variables {
    project_id                            = "espacigo-testprod"
    region                                = "southamerica-west1"
    run_id                                = "pilot01"
    enable_scratch                        = true
    enable_job                            = true
    confirm_project_inventory             = true
    confirm_image_pushed                  = true
    confirm_scratch_secret_version        = true
    confirm_database_owner_secret_version = true
    migrator_secret_version               = "4"
    database_owner_secret_version         = "1"
    migrator_image                        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                       = "user:operator@example.invalid"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.migrator) == 5 &&
      alltrue([for job in values(google_cloud_run_v2_job.migrator) : job.deletion_protection])
    )
    error_message = "Replay Jobs must start protected before their independent cleanup gate is used."
  }
}

run "replay_job_cleanup_stage1_keeps_scratch_protected" {
  command   = apply
  state_key = "replay-job-cleanup"

  variables {
    project_id                            = "espacigo-testprod"
    region                                = "southamerica-west1"
    run_id                                = "pilot01"
    enable_scratch                        = true
    enable_job                            = true
    confirm_project_inventory             = true
    confirm_image_pushed                  = true
    confirm_scratch_secret_version        = true
    confirm_database_owner_secret_version = true
    migrator_secret_version               = "4"
    database_owner_secret_version         = "1"
    migrator_image                        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    allow_replay_job_destroy              = true
    confirm_replay_job_destroy_run_id     = "pilot01"
    allow_scratch_destroy                 = false
    operator_member                       = "user:operator@example.invalid"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.migrator) == 5 &&
      alltrue([for job in values(google_cloud_run_v2_job.migrator) : !job.deletion_protection]) &&
      google_sql_database_instance.scratch[0].deletion_protection &&
      google_sql_database_instance.scratch[0].settings[0].deletion_protection_enabled &&
      google_secret_manager_secret.scratch_password[0].deletion_protection &&
      google_secret_manager_secret.scratch_database_owner_password[0].deletion_protection &&
      !var.allow_scratch_destroy
    )
    error_message = "Replay Job stage 1 may lower protection only on those Jobs; SQL and secret metadata remain protected."
  }
}

run "replay_job_cleanup_stage2_destroys_only_jobs" {
  command   = plan
  state_key = "replay-job-cleanup"

  variables {
    project_id                            = "espacigo-testprod"
    region                                = "southamerica-west1"
    run_id                                = "pilot01"
    enable_scratch                        = true
    enable_job                            = false
    confirm_project_inventory             = true
    confirm_image_pushed                  = true
    confirm_scratch_secret_version        = true
    confirm_database_owner_secret_version = true
    migrator_secret_version               = "4"
    database_owner_secret_version         = "1"
    migrator_image                        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    allow_replay_job_destroy              = true
    confirm_replay_job_destroy_run_id     = "pilot01"
    allow_scratch_destroy                 = false
    operator_member                       = "user:operator@example.invalid"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.migrator) == 0 &&
      length(google_cloud_run_v2_job_iam_member.invoke_operator) == 0 &&
      length(google_service_account_iam_member.operator_act_as) == 0 &&
      length(google_service_account_iam_member.operator_act_as_database_acl) == 0 &&
      length(google_sql_database_instance.scratch) == 1 &&
      google_sql_database_instance.scratch[0].deletion_protection &&
      google_sql_database_instance.scratch[0].settings[0].deletion_protection_enabled &&
      length(google_sql_database.scratch) == 1 &&
      google_secret_manager_secret.scratch_password[0].deletion_protection &&
      google_secret_manager_secret.scratch_database_owner_password[0].deletion_protection &&
      !var.allow_scratch_destroy
    )
    error_message = "Replay Job stage 2 may remove Jobs and their invocation/actAs bindings only, leaving SQL and secret resources intact."
  }
}

run "replay_job_cleanup_gate_requires_exact_run_confirmation" {
  command = plan

  variables {
    project_id                        = "espacigo-testprod"
    run_id                            = "pilot01"
    enable_scratch                    = true
    confirm_project_inventory         = true
    allow_replay_job_destroy          = true
    confirm_replay_job_destroy_run_id = "wrong"
  }

  expect_failures = [var.allow_replay_job_destroy]
}

run "migration_jobs_reject_latest_migrator_secret_version" {
  command = plan

  variables {
    project_id              = "espacigo-testprod"
    migrator_secret_version = "latest"
  }

  expect_failures = [var.migrator_secret_version]
}

run "migration_jobs_reject_zero_migrator_secret_version" {
  command = plan

  variables {
    project_id              = "espacigo-testprod"
    migrator_secret_version = "0"
  }

  expect_failures = [var.migrator_secret_version]
}

run "migration_jobs_reject_latest_owner_secret_version" {
  command = plan

  variables {
    project_id                    = "espacigo-testprod"
    database_owner_secret_version = "latest"
  }

  expect_failures = [var.database_owner_secret_version]
}

run "migration_jobs_reject_zero_owner_secret_version" {
  command = plan

  variables {
    project_id                    = "espacigo-testprod"
    database_owner_secret_version = "0"
  }

  expect_failures = [var.database_owner_secret_version]
}

run "role_comment_probe_jobs_require_existing_numeric_version_and_grant_only_job_invocation" {
  command = plan

  variables {
    project_id                        = "espacigo-testprod"
    region                            = "southamerica-west1"
    run_id                            = "pilot01"
    enable_scratch                    = true
    enable_role_probe_jobs            = true
    confirm_project_inventory         = true
    role_probe_image                  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_role_probe_image_pushed   = true
    role_probe_secret_version         = "1"
    confirm_role_probe_secret_version = true
    operator_member                   = "user:operator@example.invalid"
  }

  assert {
    condition = toset(keys(google_cloud_run_v2_job.role_comment_probe)) == toset([
      "role_comment_prepare", "role_comment_after_reconcile"
    ])
    error_message = "The role-comment probe must create only its two named fixed phases."
  }

  assert {
    condition = alltrue([
      for phase, job in google_cloud_run_v2_job.role_comment_probe :
      job.name == "espacigo-gcp03-pilot01-${replace(phase, "_", "-")}" &&
      can(regex("^[a-z][a-z0-9-]{0,61}[a-z0-9]$", job.name)) &&
      job.deletion_protection &&
      job.template[0].task_count == 1 &&
      job.template[0].parallelism == 1 &&
      job.template[0].template[0].max_retries == 0 &&
      job.template[0].template[0].timeout == "600s" &&
      job.template[0].template[0].containers[0].image == var.role_probe_image &&
      anytrue([for env in job.template[0].template[0].containers[0].env : env.name == "GCP03_PHASE" && env.value == phase]) &&
      anytrue([for env in job.template[0].template[0].containers[0].env : env.name == "DATABASE_URL" && env.value == "postgres://espacigo_scratch_migrator@localhost:5432/espacigo_scratch?sslmode=disable"]) &&
      anytrue([for item in job.template[0].template[0].volumes[0].secret[0].items : item.version == "1"])
    ])
    error_message = "Probe Jobs must use the exact scratch target, immutable image/secret version, one task, no retries, and a ten-minute timeout."
  }

  assert {
    condition = (
      length(google_project_iam_custom_role.role_probe_cloudsql_operator) == 0 &&
      length(google_project_iam_custom_role.role_probe_secret_operator) == 0 &&
      length(google_project_iam_member.role_probe_cloudsql_operator) == 0 &&
      length(google_secret_manager_secret_iam_member.role_probe_secret_operator) == 0 &&
      length(google_cloud_run_v2_job_iam_member.role_probe_operator) == 2 &&
      google_service_account_iam_member.operator_act_as[0].member == "user:operator@example.invalid"
    )
    error_message = "The Job gate must grant only invocation/actAs and must not grant credential-provisioning IAM."
  }
}

run "role_comment_probe_credential_gate_is_separate_project_scoped_and_expires" {
  command = plan

  variables {
    project_id                               = "espacigo-testprod"
    region                                   = "southamerica-west1"
    run_id                                   = "pilot01"
    enable_scratch                           = true
    enable_role_probe_credential_permissions = true
    confirm_project_inventory                = true
    confirm_project_scoped_cloudsql_user_ops = true
    role_probe_credential_iam_expires_at     = "2026-10-16T12:00:00Z"
    operator_member                          = "user:operator@example.invalid"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.role_comment_probe) == 0 &&
      toset(google_project_iam_custom_role.role_probe_cloudsql_operator[0].permissions) == toset([
        "cloudsql.instances.get", "cloudsql.users.create", "cloudsql.users.delete", "cloudsql.users.get", "cloudsql.users.update"
      ]) &&
      toset(google_project_iam_custom_role.role_probe_secret_operator[0].permissions) == toset([
        "secretmanager.versions.add", "secretmanager.versions.destroy", "secretmanager.versions.get", "secretmanager.versions.list"
      ]) &&
      google_project_iam_member.role_probe_cloudsql_operator[0].member == "user:operator@example.invalid" &&
      google_project_iam_member.role_probe_cloudsql_operator[0].condition[0].expression == "request.time < timestamp('2026-10-16T12:00:00Z')" &&
      google_secret_manager_secret_iam_member.role_probe_secret_operator[0].secret_id == "espacigo-gcp03-pilot01-migrator-password" &&
      google_secret_manager_secret_iam_member.role_probe_secret_operator[0].condition[0].expression == "request.time < timestamp('2026-10-16T12:00:00Z')"
    )
    error_message = "Credential provisioning must be separately gated, narrowly permissioned, project-scoped for Cloud SQL user methods, and time-bound."
  }
}

run "role_comment_probe_credential_gate_requires_project_scope_risk_acknowledgment" {
  command = plan

  variables {
    project_id                               = "espacigo-testprod"
    region                                   = "southamerica-west1"
    run_id                                   = "pilot01"
    enable_scratch                           = true
    enable_role_probe_credential_permissions = true
    confirm_project_inventory                = true
    role_probe_credential_iam_expires_at     = "2026-10-16T12:00:00Z"
    operator_member                          = "user:operator@example.invalid"
  }

  expect_failures = [var.enable_role_probe_credential_permissions]
}

run "role_comment_probe_rejects_latest_secret_version" {
  command = plan

  variables {
    project_id                        = "espacigo-testprod"
    region                            = "southamerica-west1"
    run_id                            = "pilot01"
    enable_scratch                    = true
    enable_role_probe_jobs            = true
    confirm_project_inventory         = true
    role_probe_image                  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_role_probe_image_pushed   = true
    role_probe_secret_version         = "latest"
    confirm_role_probe_secret_version = true
    operator_member                   = "user:operator@example.invalid"
  }

  expect_failures = [var.role_probe_secret_version]
}

run "role_probe_job_destroy_gate_does_not_lower_cloudsql_protection" {
  command = plan

  variables {
    project_id                            = "espacigo-testprod"
    region                                = "southamerica-west1"
    run_id                                = "pilot01"
    enable_scratch                        = true
    enable_role_probe_jobs                = true
    confirm_project_inventory             = true
    role_probe_image                      = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_role_probe_image_pushed       = true
    role_probe_secret_version             = "1"
    confirm_role_probe_secret_version     = true
    allow_role_probe_job_destroy          = true
    confirm_role_probe_job_destroy_run_id = "pilot01"
    allow_scratch_destroy                 = false
    operator_member                       = "user:operator@example.invalid"
  }

  assert {
    condition = (
      alltrue([for job in values(google_cloud_run_v2_job.role_comment_probe) : !job.deletion_protection]) &&
      google_sql_database_instance.scratch[0].deletion_protection &&
      google_sql_database_instance.scratch[0].settings[0].deletion_protection_enabled &&
      !var.allow_scratch_destroy
    )
    error_message = "The dedicated probe cleanup gate may lower protection only on the two probe Jobs; Cloud SQL protection must remain enabled."
  }
}

run "role_probe_job_destroy_gate_requires_exact_run_confirmation" {
  command = plan

  variables {
    project_id                            = "espacigo-testprod"
    region                                = "southamerica-west1"
    run_id                                = "pilot01"
    enable_scratch                        = true
    enable_role_probe_jobs                = true
    confirm_project_inventory             = true
    role_probe_image                      = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_role_probe_image_pushed       = true
    role_probe_secret_version             = "1"
    confirm_role_probe_secret_version     = true
    allow_role_probe_job_destroy          = true
    confirm_role_probe_job_destroy_run_id = "wrong"
    operator_member                       = "user:operator@example.invalid"
  }

  expect_failures = [var.allow_role_probe_job_destroy]
}

run "job_requires_enabled_scratch_and_external_artifacts" {
  command = plan

  variables {
    project_id                            = "espacigo-testprod"
    run_id                                = "pilot01"
    enable_scratch                        = false
    enable_job                            = true
    migrator_image                        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/repo/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_image_pushed                  = true
    confirm_scratch_secret_version        = true
    confirm_database_owner_secret_version = true
    operator_member                       = "user:operator@example.invalid"
  }

  expect_failures = [var.enable_job]
}

run "job_rejects_a_different_scratch_run_id" {
  command = plan

  variables {
    project_id                            = "espacigo-testprod"
    run_id                                = "pilot02"
    enable_scratch                        = true
    enable_job                            = true
    confirm_project_inventory             = true
    confirm_image_pushed                  = true
    confirm_scratch_secret_version        = true
    confirm_database_owner_secret_version = true
    migrator_image                        = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot02/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                       = "user:operator@example.invalid"
  }

  expect_failures = [var.enable_job]
}

run "teardown_gate_requires_the_exact_run_id" {
  command = plan

  variables {
    project_id             = "espacigo-testprod"
    run_id                 = "pilot01"
    allow_scratch_destroy  = true
    confirm_destroy_run_id = "another-run"
  }

  expect_failures = [var.confirm_destroy_run_id]
}

run "scratch_rejects_unreviewed_tier" {
  command = plan

  variables {
    project_id                = "espacigo-testprod"
    run_id                    = "pilot01"
    enable_scratch            = true
    confirm_project_inventory = true
    instance_tier             = "db-custom-N2-2-4096"
  }

  expect_failures = [var.instance_tier]
}

run "scratch_database_defaults_are_disposable_and_protected" {
  command = plan

  variables {
    project_id                = "espacigo-testprod"
    run_id                    = "pilot01"
    enable_scratch            = true
    confirm_project_inventory = true
  }

  assert {
    condition     = google_sql_database_instance.scratch[0].deletion_protection && google_sql_database_instance.scratch[0].settings[0].deletion_protection_enabled
    error_message = "The scratch SQL resource must require both Terraform and native deletion protection by default."
  }

  assert {
    condition     = google_sql_database_instance.scratch[0].settings[0].backup_configuration[0].enabled == false
    error_message = "Disposable scratch SQL must not create backups by default."
  }

  assert {
    condition = (
      google_sql_database_instance.scratch[0].settings[0].tier == "db-custom-N4-2-4096" &&
      google_sql_database_instance.scratch[0].settings[0].disk_type == "HYPERDISK_BALANCED"
    )
    error_message = "The reviewed N4 scratch tier must use Hyperdisk Balanced, not SSD."
  }

  assert {
    condition = (
      !google_sql_database_instance.scratch[0].settings[0].ip_configuration[0].ipv4_enabled &&
      google_sql_database_instance.scratch[0].settings[0].ip_configuration[0].ssl_mode == "ENCRYPTED_ONLY" &&
      google_sql_database_instance.scratch[0].settings[0].connector_enforcement == "REQUIRED"
    )
    error_message = "Scratch SQL must be private, require the connector, and enforce encrypted transport."
  }

  assert {
    condition     = google_secret_manager_secret.scratch_password[0].deletion_protection && google_secret_manager_secret.scratch_password[0].deletion_policy == "PREVENT"
    error_message = "The scratch-only secret container must be deletion protected until the exact run_id teardown gate is used."
  }
}
