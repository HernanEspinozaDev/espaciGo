mock_provider "google" {
  mock_data "google_compute_network" {
    defaults = {
      id   = "projects/espacigo-testprod/global/networks/espacigo-testprod"
      name = "espacigo-testprod"
    }
  }

  mock_data "google_compute_subnetwork" {
    defaults = {
      id                       = "projects/espacigo-testprod/regions/southamerica-west1/subnetworks/espacigo-testprod-serverless"
      name                     = "espacigo-testprod-serverless"
      region                   = "southamerica-west1"
      private_ip_google_access = true
    }
  }

  mock_data "google_sql_database_instance" {
    defaults = {
      connection_name    = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
      region             = "southamerica-west1"
      private_ip_address = "10.189.0.3"
    }
  }

  mock_data "google_cloud_run_v2_service" {
    defaults = { name = "espacigo-test-api" }
  }

  mock_data "google_storage_bucket" {
    defaults = { name = "g03o-v64-test-bucket" }
  }

  mock_data "google_secret_manager_secret" {
    defaults = { secret_id = "g03o-v64-test-secret" }
  }
}

run "preflight_omits_transport_secret_entirely" {
  command = plan

  variables {
    enable_job                      = true
    confirm_job_release             = true
    transfer_mode                   = "preflight"
    confirm_source_secrets          = true
    confirm_target_migrator_secret  = true
    confirm_target_files_bucket     = true
    source_admin_secret_version     = "7"
    source_transport_secret_version = ""
    target_migrator_secret_version  = "8"
    receipt_sha256                  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    receipt_generation              = "1"
    receipt_bytes                   = 128
    manifest_sha256                 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    bootstrap_report_sha256         = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    observer_report_sha256          = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    normalization_sql_sha256        = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    transfer_image                  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/g03o-v64-transfer@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    proxy_image                     = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }

  override_data {
    target = data.google_sql_database_instance.source[0]
    values = {
      connection_name = "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg"
    }
  }

  assert {
    condition     = length(data.google_secret_manager_secret.source_transport) == 0 && length(google_secret_manager_secret_iam_member.source_transport_reader) == 0
    error_message = "preflight must not look up the transport secret or grant its accessor role"
  }

  assert {
    condition     = length([for item in google_cloud_run_v2_job.transfer[0].template[0].template[0].containers[0].env : item if item.name == "GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE"]) == 0
    error_message = "preflight must not receive a transport-key file path"
  }

  assert {
    condition     = length([for item in google_cloud_run_v2_job.transfer[0].template[0].template[0].containers[0].volume_mounts : item if item.name == "source-transport"]) == 0 && length([for item in google_cloud_run_v2_job.transfer[0].template[0].template[0].volumes : item if item.name == "source-transport"]) == 0
    error_message = "preflight Job must not mount the source transport secret"
  }
}

run "execute_includes_transport_secret_access_and_mount" {
  command = plan

  variables {
    enable_job                      = true
    confirm_job_release             = true
    transfer_mode                   = "execute"
    confirm_transfer_execution      = true
    reviewed_release_sha256         = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
    confirm_source_secrets          = true
    confirm_target_migrator_secret  = true
    confirm_target_files_bucket     = true
    source_admin_secret_version     = "7"
    source_transport_secret_version = "9"
    target_migrator_secret_version  = "8"
    receipt_sha256                  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    receipt_generation              = "1"
    receipt_bytes                   = 128
    manifest_sha256                 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    bootstrap_report_sha256         = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    observer_report_sha256          = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
    normalization_sql_sha256        = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
    transfer_image                  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/g03o-v64-transfer@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    proxy_image                     = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }

  override_data {
    target = data.google_sql_database_instance.source[0]
    values = {
      connection_name = "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg"
    }
  }

  assert {
    condition     = length(data.google_secret_manager_secret.source_transport) == 1 && length(google_secret_manager_secret_iam_member.source_transport_reader) == 1 && google_secret_manager_secret_iam_member.source_transport_reader[0].role == "roles/secretmanager.secretAccessor"
    error_message = "execute must include exact transport-secret metadata and accessor grant"
  }

  assert {
    condition     = length([for item in google_cloud_run_v2_job.transfer[0].template[0].template[0].containers[0].env : item if item.name == "GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE" && item.value == "/secrets/source-transport/k_run"]) == 1
    error_message = "execute must receive the transport-key file path"
  }

  assert {
    condition     = length([for item in google_cloud_run_v2_job.transfer[0].template[0].template[0].containers[0].volume_mounts : item if item.name == "source-transport" && item.mount_path == "/secrets/source-transport"]) == 1 && length([for item in google_cloud_run_v2_job.transfer[0].template[0].template[0].volumes : item if item.name == "source-transport" && item.secret[0].items[0].version == "9"]) == 1
    error_message = "execute must mount the exact numeric transport-secret version"
  }
}
