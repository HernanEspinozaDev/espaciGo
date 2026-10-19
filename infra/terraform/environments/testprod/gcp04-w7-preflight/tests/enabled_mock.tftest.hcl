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

  mock_data "google_artifact_registry_repository" {
    defaults = {
      location = "southamerica-west1"
      name     = "projects/espacigo-testprod/locations/southamerica-west1/repositories/espacigo-testprod"
    }
  }

  mock_data "google_project" {
    defaults = {
      number = "123456789012"
    }
  }
}

run "enabled_preflight_plan_is_fully_wired" {
  command = plan

  variables {
    enable_preflight                    = true
    confirm_preflight_release           = true
    preflight_database_password_version = "7"
    required_connections                = 24
    timeout_seconds                     = 120
    preflight_image                     = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-preflight@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }

  assert {
    condition     = length(google_cloud_run_v2_job.preflight) == 1
    error_message = "enabled W7 plan must contain exactly one Cloud Run Job"
  }

  assert {
    condition     = google_cloud_run_v2_job.preflight[0].template[0].task_count == 1 && google_cloud_run_v2_job.preflight[0].template[0].parallelism == 1
    error_message = "enabled W7 job must remain single-task and serial"
  }

  assert {
    condition     = google_cloud_run_v2_job.preflight[0].template[0].template[0].timeout == "120s" && google_cloud_run_v2_job.preflight[0].template[0].template[0].max_retries == 0
    error_message = "enabled W7 job must use the reviewed timeout and no retries"
  }

  assert {
    condition     = google_cloud_run_v2_job.preflight[0].template[0].template[0].containers[0].image == var.preflight_image && contains(google_cloud_run_v2_job.preflight[0].template[0].template[0].containers[0].args, "24")
    error_message = "enabled W7 job must use its immutable image and explicit connection budget"
  }

  assert {
    condition     = google_cloud_run_v2_job.preflight[0].template[0].template[0].volumes[0].secret[0].secret == var.preflight_database_password_secret_id && google_cloud_run_v2_job.preflight[0].template[0].template[0].volumes[0].secret[0].items[0].version == "7"
    error_message = "enabled W7 job must mount the exact numeric password secret version"
  }

  assert {
    condition     = google_cloud_run_v2_job.preflight[0].template[0].template[0].vpc_access[0].network_interfaces[0].network == data.google_compute_network.existing[0].id && google_cloud_run_v2_job.preflight[0].template[0].template[0].vpc_access[0].network_interfaces[0].subnetwork == data.google_compute_subnetwork.existing[0].id
    error_message = "enabled W7 job must use the mocked existing private network and subnetwork"
  }

  assert {
    condition     = length(google_project_iam_member.preflight_cloudsql_client) == 1 && google_project_iam_member.preflight_cloudsql_client[0].condition[0].expression == "resource.name == 'projects/espacigo-testprod/instances/espacigo-testprod-pg' && resource.type == 'sqladmin.googleapis.com/Instance'" && length(google_secret_manager_secret_iam_member.preflight_password) == 1 && google_secret_manager_secret_iam_member.preflight_password[0].role == "roles/secretmanager.secretAccessor" && length(google_artifact_registry_repository_iam_member.cloud_run_reader) == 1
    error_message = "enabled W7 job must receive only its planned connector and password-secret access bindings"
  }
}

run "enabled_preflight_rejects_changed_instance_ip" {
  command = plan

  variables {
    enable_preflight                    = true
    confirm_preflight_release           = true
    preflight_database_password_version = "7"
    required_connections                = 24
    timeout_seconds                     = 120
    preflight_image                     = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-preflight@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }

  override_data {
    target = data.google_sql_database_instance.existing[0]
    values = {
      connection_name    = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
      region             = "southamerica-west1"
      private_ip_address = "10.189.0.4"
    }
  }

  expect_failures = [google_cloud_run_v2_job.preflight]
}
