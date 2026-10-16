mock_provider "google" {
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
    condition     = length(google_sql_database_instance.scratch) == 0 && length(google_cloud_run_v2_job.migrator) == 0 && length(google_cloud_run_v2_job.role_comment_probe) == 0 && length(google_project_iam_custom_role.role_probe_cloudsql_operator) == 0 && length(google_project_iam_custom_role.role_probe_secret_operator) == 0
    error_message = "Scratch database and migration job must remain disabled by default."
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
