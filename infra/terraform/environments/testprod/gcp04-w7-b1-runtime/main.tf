locals {
  enabled         = var.enable_runtime && var.confirm_release
  sql_instance    = "projects/${var.project_id}/instances/espacigo-testprod-pg"
  marker_object   = "gcp04-w7-b1/runs/${var.run_id}/probe.once.json"
  marker_resource = "projects/_/buckets/${var.marker_bucket_name}/objects/${local.marker_object}"
  release_payload = try(jsondecode(var.release_json), {})
  release_pins_match = try(
    local.release_payload.release.decision == "approved" &&
    local.release_payload.release.stage == "b1_probe" &&
    local.release_payload.release.release_id == var.run_id &&
    local.release_payload.release.create_operation_id != "" &&
    local.release_payload.release.resource_release_id == var.run_id &&
    local.release_payload.release.reviewer == "HernanEspinozaDev" &&
    local.release_payload.release.expires_at != "" &&
    local.release_payload.release.max_duration_seconds > 0 &&
    local.release_payload.release.max_duration_seconds <= 1800 &&
    local.release_payload.release.target == "espacigo-testprod/espacigo-testprod-pg" &&
    local.release_payload.release.login == "espacigo_gcp04_w7_bootstrap" &&
    local.release_payload.release.database_role == "cloudsqlsuperuser" &&
    local.release_payload.release.secret_id == var.database_password_secret_id &&
    local.release_payload.release.secret_version == var.database_password_secret_version &&
    local.release_payload.release.secret_etag != "" &&
    local.release_payload.release.secret_version_etag != "" &&
    local.release_payload.release.query_sha256 == "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826" &&
    local.release_payload.release.runs == 1 &&
    local.release_payload.pins.image_digest == var.image_digest &&
    local.release_payload.pins.binary_sha256 == var.binary_sha256,
    false
  )
  runtime_ready = var.run_id != "" && var.image_digest != "" && var.database_password_secret_version != "" && var.release_json != "" && var.release_sha256 != "" && var.marker_bucket_name != "" && var.binary_sha256 != "" && local.release_pins_match && sha256(var.release_json) == var.release_sha256
}

data "google_compute_network" "target" {
  count   = local.enabled ? 1 : 0
  name    = var.network_name
  project = var.project_id
}

data "google_compute_subnetwork" "target" {
  count   = local.enabled ? 1 : 0
  name    = var.subnetwork_name
  region  = var.region
  project = var.project_id
}

data "google_storage_bucket" "run_marker" {
  count = local.enabled ? 1 : 0
  name  = var.marker_bucket_name
}

resource "google_service_account" "runtime" {
  count        = local.enabled ? 1 : 0
  project      = var.project_id
  account_id   = var.service_account_id
  display_name = "EspaciGo W7 B1 one-run catalog probe"
  description  = "Read-only B1 probe runtime; no Cloud SQL Admin or Secret Manager metadata API access."
}

resource "google_project_iam_member" "cloudsql_client" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.runtime[0].email}"
  condition {
    title       = "gcp04-w7-b1-exact-instance"
    description = "Cloud SQL connector access for the fixed EspaciGo instance only."
    expression  = "resource.name == '${local.sql_instance}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_secret_manager_secret_iam_member" "credential_accessor" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = var.database_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime[0].email}"
}


resource "google_storage_bucket_iam_member" "run_marker_creator" {
  count  = local.enabled ? 1 : 0
  bucket = var.marker_bucket_name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.runtime[0].email}"

  condition {
    title       = "gcp04-w7-b1-exact-once-marker-${var.run_id}"
    description = "May create only this run's immutable single-use marker."
    expression  = "resource.name == '${local.marker_resource}'"
  }
}

resource "google_cloud_run_v2_job" "probe" {
  count               = local.enabled ? 1 : 0
  project             = var.project_id
  name                = "${var.job_name}-${var.run_id}"
  location            = var.region
  deletion_protection = false
  depends_on          = [google_project_iam_member.cloudsql_client, google_secret_manager_secret_iam_member.credential_accessor, google_storage_bucket_iam_member.run_marker_creator]

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.runtime[0].email
      max_retries     = 0
      timeout         = "120s"

      volumes {
        name = "b1-password"
        secret {
          secret = var.database_password_secret_id
          items {
            version = var.database_password_secret_version
            path    = "password"
            mode    = 256 # root-owned 0400, copied to a private 0600 temp file by the runtime
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.target[0].id
          subnetwork = data.google_compute_subnetwork.target[0].id
          tags       = ["espacigo-gcp04-w7-b1-${var.run_id}"]
        }
      }

      containers {
        image = var.image_digest
        resources {
          limits = { cpu = "1", memory = "512Mi" }
        }
        env {
          name  = "GCP04_B1_RELEASE_JSON"
          value = var.release_json
        }
        env {
          name  = "GCP04_B1_RELEASE_SHA256"
          value = var.release_sha256
        }
        env {
          name  = "GCP04_B1_SECRET_VERSION"
          value = var.database_password_secret_version
        }
        env {
          name  = "GCP04_B1_IMAGE_DIGEST"
          value = var.image_digest
        }
        env {
          name  = "GCP04_B1_BINARY_SHA256"
          value = var.binary_sha256
        }
        env {
          name  = "GCP04_B1_MARKER_BUCKET"
          value = var.marker_bucket_name
        }
        env {
          name  = "GCP04_B1_MARKER_PREFIX"
          value = "gcp04-w7-b1/runs"
        }
        volume_mounts {
          name       = "b1-password"
          mount_path = "/var/run/secrets/b1"
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition     = local.runtime_ready && var.project_id == "espacigo-testprod" && var.region == "southamerica-west1" && lower(data.google_storage_bucket.run_marker[0].location) == var.region
      error_message = "W7 B1 runtime requires an exact run-scoped release, matching pins and a same-region marker bucket."
    }
    precondition {
      condition     = local.enabled
      error_message = "Both enable_runtime and confirm_release must be true for any runtime resources."
    }
  }
}
