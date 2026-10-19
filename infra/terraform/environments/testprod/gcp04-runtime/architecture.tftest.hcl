mock_provider "google" {
  mock_data "google_compute_network" {
    defaults = {
      id      = "projects/espacigo-testprod/global/networks/espacigo-testprod"
      name    = "espacigo-testprod"
      project = "espacigo-testprod"
    }
  }

  mock_data "google_compute_subnetwork" {
    defaults = {
      id                       = "projects/espacigo-testprod/regions/southamerica-west1/subnetworks/espacigo-testprod-serverless"
      name                     = "espacigo-testprod-serverless"
      project                  = "espacigo-testprod"
      region                   = "southamerica-west1"
      private_ip_google_access = true
    }
  }

  mock_data "google_sql_database_instance" {
    defaults = {
      connection_name = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
      name            = "espacigo-testprod-pg"
      project         = "espacigo-testprod"
      region          = "southamerica-west1"
    }
  }

  mock_data "google_artifact_registry_repository" {
    defaults = {
      name          = "projects/espacigo-testprod/locations/southamerica-west1/repositories/espacigo-testprod"
      location      = "southamerica-west1"
      project       = "espacigo-testprod"
      repository_id = "espacigo-testprod"
    }
  }

  mock_data "google_project" {
    defaults = {
      project_id = "espacigo-testprod"
      number     = "123456789012"
    }
  }

  mock_resource "google_service_account" {
    defaults = {
      email = "espacigo-test-privacy-worker@espacigo-testprod.iam.gserviceaccount.com"
      name  = "projects/espacigo-testprod/serviceAccounts/espacigo-test-privacy-worker@espacigo-testprod.iam.gserviceaccount.com"
    }
  }
}

run "enabled_worker_and_migrator_use_readable_secret_files" {
  command = plan

  variables {
    enable_worker                        = true
    enable_migrator                      = true
    confirm_runtime_database_bootstrap   = true
    confirm_connection_budget            = true
    confirm_cost_review                  = true
    artifact_registry_repository_id      = "espacigo-testprod"
    worker_database_user                 = "espacigo_privacy_worker_login"
    migrator_database_user               = "espacigo_migrator_login"
    worker_database_password_secret_id   = "privacy-worker-db-password"
    worker_database_password_version     = "7"
    migrator_database_password_secret_id = "migrator-db-password"
    migrator_database_password_version   = "11"
    migrator_service_account_email       = "espacigo-migrator@espacigo-testprod.iam.gserviceaccount.com"
    private_bucket_name                  = "espacigo-test-private-files"
    worker_image                         = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/privacy-worker@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    migrator_image                       = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/runtime-migrator@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }

  assert {
    condition     = length(google_cloud_run_v2_worker_pool.privacy_cleanup) == 1 && length(google_cloud_run_v2_job.migrator) == 1
    error_message = "The enabled mock plan must include both private runtime workloads."
  }

  assert {
    condition = (google_cloud_run_v2_worker_pool.privacy_cleanup[0].template[0].volumes[0].secret[0].items[0].version == "7" &&
      google_cloud_run_v2_worker_pool.privacy_cleanup[0].template[0].volumes[0].secret[0].items[0].mode == 292 &&
      one([for env in google_cloud_run_v2_worker_pool.privacy_cleanup[0].template[0].containers[0].env : env.value if env.name == "DATABASE_PASSWORD_FILE"]) == "/var/secrets/database-password/password" &&
      google_cloud_run_v2_worker_pool.privacy_cleanup[0].template[0].containers[0].volume_mounts[0].name == "database-password" &&
    google_cloud_run_v2_worker_pool.privacy_cleanup[0].template[0].containers[0].volume_mounts[0].mount_path == "/var/secrets/database-password")
    error_message = "The worker must mount its numeric-version secret as a readable file at DATABASE_PASSWORD_FILE for UID 65532."
  }

  assert {
    condition = (google_cloud_run_v2_job.migrator[0].template[0].template[0].volumes[0].secret[0].items[0].version == "11" &&
      google_cloud_run_v2_job.migrator[0].template[0].template[0].volumes[0].secret[0].items[0].mode == 292 &&
      one([for env in google_cloud_run_v2_job.migrator[0].template[0].template[0].containers[0].env : env.value if env.name == "DATABASE_PASSWORD_FILE"]) == "/var/secrets/database-password/password" &&
      google_cloud_run_v2_job.migrator[0].template[0].template[0].containers[0].volume_mounts[0].name == "database-password" &&
    google_cloud_run_v2_job.migrator[0].template[0].template[0].containers[0].volume_mounts[0].mount_path == "/var/secrets/database-password")
    error_message = "The migrator must mount its numeric-version secret as a readable file at DATABASE_PASSWORD_FILE for UID 65532."
  }

  assert {
    condition = (length(google_artifact_registry_repository_iam_member.cloud_run_reader) == 1 &&
      length(google_project_iam_member.worker_cloudsql_client) == 1 &&
      length(google_storage_bucket_iam_member.worker_private_files) == 1 &&
      length(google_secret_manager_secret_iam_member.worker_db_password) == 1 &&
      length(google_project_iam_member.migrator_cloudsql_client) == 1 &&
    length(google_secret_manager_secret_iam_member.migrator_db_password) == 1)
    error_message = "The enabled mock plan must include the workload IAM grants referenced by depends_on."
  }
}
