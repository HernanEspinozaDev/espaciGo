mock_provider "google" {
  override_during = plan

  mock_data "google_compute_network" {
    defaults = {
      self_link = "projects/espacigo-testprod/global/networks/espacigo-testprod"
      name      = "espacigo-testprod"
    }
  }

  mock_data "google_compute_subnetwork" {
    defaults = {
      name                     = "espacigo-testprod-serverless"
      private_ip_google_access = true
      region                   = "southamerica-west1"
    }
  }

  mock_data "google_project" {
    defaults = { number = "447373903153" }
  }

  mock_resource "google_sql_database_instance" {
    defaults = {
      connection_name = "espacigo-testprod:southamerica-west1:espacigo-gcp04-rtest1-pg"
    }
  }
}

run "all_cloud_resources_are_off_by_default" {
  command = plan

  variables {
    project_id = "espacigo-testprod"
    region     = "southamerica-west1"
  }

  assert {
    condition = (
      length(google_sql_database_instance.scratch) == 0 &&
      length(google_storage_bucket.private) == 0 &&
      length(google_cloud_run_v2_job.smoke) == 0 &&
      length(google_service_account.runner) == 0 &&
      length(google_secret_manager_secret.database_owner_password) == 0 &&
      length(google_artifact_registry_repository.smoke) == 0
    )
    error_message = "GCP-04 smoke must not create resources or enable APIs unless separately gated."
  }
}

run "base_is_one_isolated_private_database_and_bucket_without_a_job" {
  command = plan

  variables {
    project_id                 = "espacigo-testprod"
    region                     = "southamerica-west1"
    run_id                     = "rtest1"
    bucket_suffix              = "smoke-rtest1"
    enable_base                = true
    confirm_isolated_inventory = true
  }

  assert {
    condition = (
      length(google_sql_database_instance.scratch) == 1 &&
      google_sql_database_instance.scratch[0].database_version == "POSTGRES_18" &&
      google_sql_database_instance.scratch[0].deletion_protection &&
      google_sql_database_instance.scratch[0].settings[0].ip_configuration[0].ipv4_enabled == false &&
      google_sql_database_instance.scratch[0].settings[0].disk_size == 20 &&
      length(google_storage_bucket.private) == 1 &&
      google_storage_bucket.private[0].public_access_prevention == "enforced" &&
      google_storage_bucket.private[0].uniform_bucket_level_access &&
      !google_storage_bucket.private[0].force_destroy &&
      length(google_cloud_run_v2_job.smoke) == 0 &&
      length(google_project_iam_member.runner_cloudsql_client) == 1
    )
    error_message = "The base must contain one protected private SQL 18 instance and one private bucket, without running a Job."
  }
}

run "smoke_job_is_single_task_digest_pinned_and_has_no_retries" {
  command = plan

  variables {
    project_id                    = "espacigo-testprod"
    region                        = "southamerica-west1"
    run_id                        = "rtest1"
    bucket_suffix                 = "smoke-rtest1"
    enable_base                   = true
    enable_job                    = true
    confirm_isolated_inventory    = true
    confirm_image_digest          = true
    runner_image                  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp04-replay-rtest1/smoke@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    database_owner_secret_version = "4"
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.smoke) == 1 &&
      google_cloud_run_v2_job.smoke[0].deletion_protection &&
      google_cloud_run_v2_job.smoke[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.smoke[0].template[0].parallelism == 1 &&
      google_cloud_run_v2_job.smoke[0].template[0].template[0].max_retries == 0 &&
      google_cloud_run_v2_job.smoke[0].template[0].template[0].timeout == "900s" &&
      google_cloud_run_v2_job.smoke[0].template[0].template[0].containers[0].image == var.runner_image &&
      length(google_cloud_run_v2_job_iam_member.operator) == 1 &&
      google_cloud_run_v2_job_iam_member.operator[0].member == var.operator_member &&
      google_cloud_run_v2_job_iam_member.operator[0].role == "roles/run.invoker"
    )
    error_message = "The smoke Job must be a protected one-shot digest-pinned task with no retries and one exact operator binding."
  }
}
