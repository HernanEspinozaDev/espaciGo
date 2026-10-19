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
  mock_data "google_storage_bucket" {
    defaults = {
      name     = "private-w7-runmarks"
      location = "SOUTHAMERICA-WEST1"
    }
  }
}

run "enabled_runtime_uses_one_claim_and_exact_release_pins" {
  command = plan
  variables {
    enable_runtime                   = true
    confirm_release                  = true
    run_id                           = "b1-run-1"
    marker_bucket_name               = "private-w7-runmarks"
    database_password_secret_version = "7"
    image_digest                     = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    binary_sha256                    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    release_json = jsonencode({
      release = {
        decision     = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1", create_operation_id = "create-op-1", secret_etag = "secret-etag"
        expires_at   = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        reviewer     = "HernanEspinozaDev", target = "espacigo-testprod/espacigo-testprod-pg"
        login        = "espacigo_gcp04_w7_bootstrap", database_role = "cloudsqlsuperuser"
        secret_id    = "espacigo-gcp04-w7-b1-bootstrap-credential", secret_version = "7", secret_version_etag = "etag-7"
        query_sha256 = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        binary_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    })
    release_sha256 = sha256(jsonencode({
      release = {
        decision     = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1", create_operation_id = "create-op-1", secret_etag = "secret-etag"
        expires_at   = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        reviewer     = "HernanEspinozaDev", target = "espacigo-testprod/espacigo-testprod-pg"
        login        = "espacigo_gcp04_w7_bootstrap", database_role = "cloudsqlsuperuser"
        secret_id    = "espacigo-gcp04-w7-b1-bootstrap-credential", secret_version = "7", secret_version_etag = "etag-7"
        query_sha256 = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        binary_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    }))
  }

  assert {
    condition     = length(google_cloud_run_v2_job.probe) == 1 && google_cloud_run_v2_job.probe[0].template[0].task_count == 1 && google_cloud_run_v2_job.probe[0].template[0].parallelism == 1 && google_cloud_run_v2_job.probe[0].template[0].template[0].max_retries == 0
    error_message = "W7 B1 runtime must be a one-task, one-parallelism, zero-retry Job."
  }
  assert {
    condition     = google_cloud_run_v2_job.probe[0].template[0].template[0].volumes[0].secret[0].items[0].version == "7" && google_cloud_run_v2_job.probe[0].template[0].template[0].volumes[0].secret[0].items[0].mode == 256
    error_message = "W7 B1 must mount the exact release version at root mode 0400."
  }
  assert {
    condition     = length(google_storage_bucket_iam_member.run_marker_creator) == 1 && google_storage_bucket_iam_member.run_marker_creator[0].role == "roles/storage.objectCreator" && google_storage_bucket_iam_member.run_marker_creator[0].condition[0].expression == "resource.name == 'projects/_/buckets/private-w7-runmarks/objects/gcp04-w7-b1/runs/b1-run-1/probe.once.json'"
    error_message = "Runtime SA must only create its exact run's marker object."
  }
}

run "enabled_runtime_rejects_secret_version_drift" {
  command = plan
  variables {
    enable_runtime                   = true
    confirm_release                  = true
    run_id                           = "b1-run-1"
    marker_bucket_name               = "private-w7-runmarks"
    database_password_secret_version = "7"
    image_digest                     = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    binary_sha256                    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    release_json = jsonencode({
      release = {
        decision     = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1", create_operation_id = "create-op-1", secret_etag = "secret-etag"
        expires_at   = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        reviewer     = "HernanEspinozaDev", target = "espacigo-testprod/espacigo-testprod-pg"
        login        = "espacigo_gcp04_w7_bootstrap", database_role = "cloudsqlsuperuser"
        secret_id    = "espacigo-gcp04-w7-b1-bootstrap-credential", secret_version = "8", secret_version_etag = "etag-8"
        query_sha256 = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        binary_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    })
    release_sha256 = sha256(jsonencode({
      release = {
        decision     = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1", create_operation_id = "create-op-1", secret_etag = "secret-etag"
        expires_at   = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        reviewer     = "HernanEspinozaDev", target = "espacigo-testprod/espacigo-testprod-pg"
        login        = "espacigo_gcp04_w7_bootstrap", database_role = "cloudsqlsuperuser"
        secret_id    = "espacigo-gcp04-w7-b1-bootstrap-credential", secret_version = "8", secret_version_etag = "etag-8"
        query_sha256 = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        binary_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    }))
  }
  expect_failures = [google_cloud_run_v2_job.probe]
}

run "enabled_runtime_rejects_query_or_release_hash_drift" {
  command = plan
  variables {
    enable_runtime                   = true
    confirm_release                  = true
    run_id                           = "b1-run-1"
    marker_bucket_name               = "private-w7-runmarks"
    database_password_secret_version = "7"
    image_digest                     = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    binary_sha256                    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    release_json = jsonencode({
      release = {
        decision     = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1", create_operation_id = "create-op-1", secret_etag = "secret-etag"
        expires_at   = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        reviewer     = "HernanEspinozaDev", target = "espacigo-testprod/espacigo-testprod-pg"
        login        = "espacigo_gcp04_w7_bootstrap", database_role = "cloudsqlsuperuser"
        secret_id    = "espacigo-gcp04-w7-b1-bootstrap-credential", secret_version = "7", secret_version_etag = "etag-7"
        query_sha256 = "wrong", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        binary_sha256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      }
    })
    release_sha256 = "0000000000000000000000000000000000000000000000000000000000000000"
  }
  expect_failures = [google_cloud_run_v2_job.probe]
}

run "enabled_runtime_rejects_image_and_binary_pin_drift" {
  command = plan
  variables {
    enable_runtime                   = true
    confirm_release                  = true
    run_id                           = "b1-run-1"
    marker_bucket_name               = "private-w7-runmarks"
    database_password_secret_version = "7"
    image_digest                     = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    binary_sha256                    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    release_json = jsonencode({
      release = {
        decision            = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1"
        create_operation_id = "create-op-1", reviewer = "HernanEspinozaDev"
        expires_at          = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        target              = "espacigo-testprod/espacigo-testprod-pg", login = "espacigo_gcp04_w7_bootstrap"
        database_role       = "cloudsqlsuperuser", secret_id = "espacigo-gcp04-w7-b1-bootstrap-credential"
        secret_version      = "7", secret_etag = "secret-etag", secret_version_etag = "etag-7"
        query_sha256        = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
        binary_sha256 = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
      }
    })
    release_sha256 = sha256(jsonencode({
      release = {
        decision            = "approved", stage = "b1_probe", release_id = "b1-run-1", resource_release_id = "b1-run-1"
        create_operation_id = "create-op-1", reviewer = "HernanEspinozaDev"
        expires_at          = "2099-10-19T00:00:00Z", max_duration_seconds = 120
        target              = "espacigo-testprod/espacigo-testprod-pg", login = "espacigo_gcp04_w7_bootstrap"
        database_role       = "cloudsqlsuperuser", secret_id = "espacigo-gcp04-w7-b1-bootstrap-credential"
        secret_version      = "7", secret_etag = "secret-etag", secret_version_etag = "etag-7"
        query_sha256        = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826", runs = 1
      }
      pins = {
        image_digest  = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
        binary_sha256 = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
      }
    }))
  }
  expect_failures = [google_cloud_run_v2_job.probe]
}
