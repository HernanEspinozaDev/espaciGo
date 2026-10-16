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
    condition     = length(google_sql_database_instance.scratch) == 0 && length(google_cloud_run_v2_job.migrator) == 0
    error_message = "Scratch database and migration job must remain disabled by default."
  }
}

run "job_creates_four_immutable_named_phases_with_exact_operator_iam" {
  command = plan

  variables {
    project_id                     = "espacigo-testprod"
    region                         = "southamerica-west1"
    run_id                         = "pilot01"
    enable_scratch                 = true
    enable_job                     = true
    confirm_project_inventory      = true
    confirm_image_pushed           = true
    confirm_scratch_secret_version = true
    migrator_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                = "user:operator@example.invalid"
  }

  assert {
    condition = toset(keys(google_cloud_run_v2_job.migrator)) == toset([
      "bootstrap", "migrate", "finalize", "retire"
    ])
    error_message = "The reviewed Job gate must create only the four named scratch phases."
  }

  assert {
    condition = alltrue([
      for phase, job in google_cloud_run_v2_job.migrator :
      job.name == "espacigo-gcp03-pilot01-${phase}" &&
      job.template[0].task_count == 1 &&
      job.template[0].parallelism == 1 &&
      job.template[0].template[0].max_retries == 0 &&
      anytrue([for env in job.template[0].template[0].containers[0].env : env.name == "GCP03_PHASE" && env.value == phase])
    ])
    error_message = "Each named Job must have its matching phase and single-task no-retry execution settings."
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job_iam_member.invoke_operator) == 4 &&
      alltrue([for binding in values(google_cloud_run_v2_job_iam_member.invoke_operator) : binding.role == "roles/run.invoker" && binding.member == "user:operator@example.invalid"]) &&
      google_service_account_iam_member.operator_act_as[0].role == "roles/iam.serviceAccountUser"
    )
    error_message = "Only the named operator should receive invoker on each phase and actAs on the scratch service account."
  }
}

run "job_requires_enabled_scratch_and_external_artifacts" {
  command = plan

  variables {
    project_id                     = "espacigo-testprod"
    run_id                         = "pilot01"
    enable_scratch                 = false
    enable_job                     = true
    migrator_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/repo/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    confirm_image_pushed           = true
    confirm_scratch_secret_version = true
    operator_member                = "user:operator@example.invalid"
  }

  expect_failures = [var.enable_job]
}

run "job_rejects_a_different_scratch_run_id" {
  command = plan

  variables {
    project_id                     = "espacigo-testprod"
    run_id                         = "pilot02"
    enable_scratch                 = true
    enable_job                     = true
    confirm_project_inventory      = true
    confirm_image_pushed           = true
    confirm_scratch_secret_version = true
    migrator_image                 = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot02/dbmigrate@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    operator_member                = "user:operator@example.invalid"
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
