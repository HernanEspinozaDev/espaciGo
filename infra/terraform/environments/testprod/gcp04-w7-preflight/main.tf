locals {
  enabled         = var.enable_preflight
  sql_instance    = "projects/${var.project_id}/instances/${var.sql_instance_name}"
  secret_resource = "projects/${var.project_id}/secrets/${var.preflight_database_password_secret_id}"
  query_sha256    = "d0d3101667aff59899e231ca4ecd27c66358fd4e83c4615aae97ab873b4c2701"
}

data "google_compute_network" "existing" {
  count   = local.enabled ? 1 : 0
  name    = var.network_name
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  count   = local.enabled ? 1 : 0
  name    = var.subnetwork_name
  region  = var.region
  project = var.project_id
}

data "google_sql_database_instance" "existing" {
  count   = local.enabled ? 1 : 0
  name    = var.sql_instance_name
  project = var.project_id
}

data "google_artifact_registry_repository" "images" {
  count         = local.enabled ? 1 : 0
  project       = var.project_id
  location      = var.region
  repository_id = var.artifact_registry_repository_id
}

data "google_project" "current" {
  count      = local.enabled ? 1 : 0
  project_id = var.project_id
}

resource "google_artifact_registry_repository_iam_member" "cloud_run_reader" {
  count      = local.enabled ? 1 : 0
  project    = var.project_id
  location   = data.google_artifact_registry_repository.images[0].location
  repository = data.google_artifact_registry_repository.images[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:service-${data.google_project.current[0].number}@serverless-robot-prod.iam.gserviceaccount.com"
}

resource "google_service_account" "preflight" {
  count        = local.enabled ? 1 : 0
  project      = var.project_id
  account_id   = var.preflight_service_account_id
  display_name = "EspaciGo GCP-04 W7 target preflight"
  description  = "One-task read-only SQL inventory; no migration or data write privileges."
}

resource "google_project_iam_member" "preflight_cloudsql_client" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.preflight[0].email}"

  condition {
    title       = "gcp04-w7-preflight-cloudsql-instance"
    description = "Connector access only to the existing EspaciGo Cloud SQL instance."
    expression  = "resource.name == '${local.sql_instance}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_secret_manager_secret_iam_member" "preflight_password" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = var.preflight_database_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.preflight[0].email}"
}

resource "google_cloud_run_v2_job" "preflight" {
  count               = local.enabled ? 1 : 0
  project             = var.project_id
  name                = var.preflight_job_name
  location            = var.region
  deletion_protection = false
  depends_on = [
    google_artifact_registry_repository_iam_member.cloud_run_reader,
    google_project_iam_member.preflight_cloudsql_client,
    google_secret_manager_secret_iam_member.preflight_password,
  ]

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.preflight[0].email
      max_retries     = 0
      timeout         = "${var.timeout_seconds}s"

      volumes {
        name = "observer-password"
        secret {
          secret = var.preflight_database_password_secret_id
          items {
            version = var.preflight_database_password_version
            path    = "password"
            mode    = 292
          }
        }
      }

      vpc_access {
        connector = null
        egress    = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].id
          subnetwork = data.google_compute_subnetwork.existing[0].id
          tags       = ["espacigo-gcp04-w7-preflight"]
        }
      }

      containers {
        image = var.preflight_image
        args = [
          "--required-connections", tostring(var.required_connections),
          "--timeout-seconds", tostring(var.timeout_seconds),
          "--concurrency", "1",
          "--query-file", "/target_preflight.sql",
        ]
        resources {
          limits = { cpu = "1", memory = "512Mi" }
        }
        env {
          name  = "GCP04_PREFLIGHT_PROJECT_ID"
          value = var.project_id
        }
        env {
          name  = "GCP04_PREFLIGHT_EXPECTED_PRIVATE_IP"
          value = var.preflight_expected_private_ip
        }
        env {
          name  = "GCP04_PREFLIGHT_SECRET_VERSION"
          value = var.preflight_database_password_version
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = data.google_sql_database_instance.existing[0].connection_name
        }
        env {
          name  = "DATABASE_NAME"
          value = var.database_name
        }
        env {
          name  = "DATABASE_USER"
          value = var.preflight_database_user
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/var/secrets/observer-password/password"
        }
        volume_mounts {
          name       = "observer-password"
          mount_path = "/var/secrets/observer-password"
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        var.confirm_preflight_release && var.project_id == "espacigo-testprod" &&
        var.preflight_database_user == "espacigo_gcp04_w7_observer" &&
        var.preflight_database_password_secret_id == "espacigo-gcp04-w7-observer-password" &&
        can(regex("^[1-9][0-9]*$", var.preflight_database_password_version)) &&
        var.required_connections >= 1 && var.preflight_expected_private_ip == "10.189.0.3" &&
        can(regex("^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp04-preflight@sha256:[0-9a-f]{64}$", var.preflight_image)) && local.query_sha256 == "d0d3101667aff59899e231ca4ecd27c66358fd4e83c4615aae97ab873b4c2701"
      )
      error_message = "W7 activation requires a separately reviewed release, exact target/login, numeric active secret version, positive connection budget and immutable image digest."
    }
    precondition {
      condition = (
        data.google_sql_database_instance.existing[0].connection_name == "espacigo-testprod:southamerica-west1:espacigo-testprod-pg" &&
        data.google_sql_database_instance.existing[0].region == var.region &&
        data.google_sql_database_instance.existing[0].private_ip_address == var.preflight_expected_private_ip &&
        data.google_compute_subnetwork.existing[0].private_ip_google_access
      )
      error_message = "W7 requires the exact current SQL connection name, region, private IP and same-region private Google access subnet."
    }
  }
}
