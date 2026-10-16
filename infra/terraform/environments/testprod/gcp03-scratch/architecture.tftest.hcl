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
