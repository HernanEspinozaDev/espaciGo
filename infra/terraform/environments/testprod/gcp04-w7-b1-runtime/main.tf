locals {
  enabled         = var.enable_runtime && var.confirm_release
  sql_instance    = "projects/${var.project_id}/instances/espacigo-testprod-pg"
  secret_resource = "projects/${var.project_id}/secrets/${var.database_password_secret_id}"
  runtime_ready   = var.run_id != "" && var.image_digest != "" && var.database_password_secret_version != "" && var.release_json != "" && var.release_sha256 != ""
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

resource "google_cloud_run_v2_job" "probe" {
  count               = local.enabled ? 1 : 0
  project             = var.project_id
  name                = "${var.job_name}-${var.run_id}"
  location            = var.region
  deletion_protection = false
  depends_on          = [google_project_iam_member.cloudsql_client, google_secret_manager_secret_iam_member.credential_accessor]

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
        volume_mounts {
          name       = "b1-password"
          mount_path = "/var/run/secrets/b1"
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition     = local.runtime_ready && var.project_id == "espacigo-testprod" && var.region == "southamerica-west1"
      error_message = "W7 B1 runtime requires a run-scoped release, fixed target, numeric secret version and immutable image digest."
    }
    precondition {
      condition     = local.enabled
      error_message = "Both enable_runtime and confirm_release must be true for any runtime resources."
    }
  }
}
