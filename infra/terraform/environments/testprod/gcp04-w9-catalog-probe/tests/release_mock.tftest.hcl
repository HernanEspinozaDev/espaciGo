mock_provider "google" {
  mock_data "google_compute_network" {
    defaults = { id = "projects/espacigo-testprod/global/networks/espacigo-testprod",
    name = "espacigo-testprod" }
  }
  mock_data "google_compute_subnetwork" {
    defaults = { id = "projects/espacigo-testprod/regions/southamerica-west1/subnetworks/espacigo-testprod-serverless",
      name = "espacigo-testprod-serverless",
    region = "southamerica-west1" }
  }
  mock_data "google_sql_database_instance" {
    defaults = { connection_name = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg",
      region           = "southamerica-west1",
      database_version = "POSTGRES_18",
    private_ip_address = "10.189.0.3" }
  }
  mock_data "google_secret_manager_secret" {
    defaults = {
      name      = "projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password"
      secret_id = "espacigo-gcp04-w8-catalog-probe-password"
      replication = [{
        user_managed = [{ replicas = [{ location = "southamerica-west1" }] }]
      }]
    }
  }
  mock_data "google_artifact_registry_repository" {
    defaults = { location = "southamerica-west1",
    name = "projects/espacigo-testprod/locations/southamerica-west1/repositories/espacigo-testprod" }
  }
  mock_data "google_project" { defaults = { number = "123456789012" } }
}

run "exact_mock_release_plan" {
  command = plan
  variables {
    enable_w9_release  = true
    confirm_w9_release = true
    reviewer           = "HernanEspinozaDev"
    release_id         = "w9-review-fixture"
    release_sha256     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    secret_version     = "7"
    image_digest       = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w8-catalog-probe@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
  assert {
    condition     = length(google_cloud_run_v2_job.probe) == 1 && google_cloud_run_v2_job.probe[0].template[0].task_count == 1 && google_cloud_run_v2_job.probe[0].template[0].parallelism == 1
    error_message = "W9 mock release must plan exactly one serial Cloud Run Job."
  }
  assert {
    condition     = google_cloud_run_v2_job.probe[0].template[0].template[0].max_retries == 0 && google_cloud_run_v2_job.probe[0].template[0].template[0].timeout == "120s" && google_cloud_run_v2_job.probe[0].template[0].template[0].volumes[0].secret[0].items[0].version == "7"
    error_message = "W9 must pin one numeric secret version, zero retries and a 120 second maximum."
  }
  assert {
    condition     = strcontains(google_project_iam_member.sql_client[0].condition[0].expression, "projects/espacigo-testprod/instances/espacigo-testprod-pg") && google_secret_manager_secret_iam_member.secret_accessor[0].role == "roles/secretmanager.secretAccessor" && strcontains(google_secret_manager_secret_iam_member.secret_accessor[0].secret_id, "espacigo-gcp04-w8-catalog-probe-password")
    error_message = "W9 IAM must target only the exact SQL instance and secret."
  }
}

run "rejects_wrong_candidate_ip" {
  command = plan
  variables {
    enable_w9_release  = true
    confirm_w9_release = true
    reviewer           = "HernanEspinozaDev"
    release_id         = "w9-review-fixture"
    release_sha256     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    secret_version     = "7"
    image_digest       = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w8-catalog-probe@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
  }
  override_data {
    target = data.google_sql_database_instance.target[0]
    values = { connection_name = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg",
      region           = "southamerica-west1",
      database_version = "POSTGRES_18",
    private_ip_address = "10.189.0.4" }
  }
  expect_failures = [google_cloud_run_v2_job.probe]
}
