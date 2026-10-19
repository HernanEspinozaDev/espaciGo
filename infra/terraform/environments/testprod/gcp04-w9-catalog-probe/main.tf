locals {
  enabled       = var.enable_w9_release
  sql_resource  = "projects/${var.project_id}/instances/${var.sql_instance_name}"
  secret_id     = "espacigo-gcp04-w8-catalog-probe-password"
  login         = "espacigo_gcp04_w8_catalog_probe"
  image_pattern = "^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp04-w8-catalog-probe@sha256:[0-9a-f]{64}$"
}

data "google_compute_network" "target" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  name    = var.network_name
}
data "google_compute_subnetwork" "target" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  region  = var.region
  name    = var.subnetwork_name
}
data "google_sql_database_instance" "target" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  name    = var.sql_instance_name
}
data "google_secret_manager_secret" "target" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = local.secret_id
}
data "google_artifact_registry_repository" "target" {
  count         = local.enabled ? 1 : 0
  project       = var.project_id
  location      = var.region
  repository_id = var.repository_id
}
data "google_project" "target" {
  count      = local.enabled ? 1 : 0
  project_id = var.project_id
}
resource "google_service_account" "probe" {
  count        = local.enabled ? 1 : 0
  project      = var.project_id
  account_id   = "espacigo-gcp04-w8-probe"
  display_name = "GCP-04 W9 one-shot catalog probe"
  description  = "One-task allowlisted read-only W8 catalog query."
}
resource "google_project_iam_member" "sql_client" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.probe[0].email}"
  condition {
    title       = "gcp04-w9-exact-sql-instance"
    description = "Connector access to exact existing SQL instance"
    expression  = "resource.name == '${local.sql_resource}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}
resource "google_secret_manager_secret_iam_member" "secret_accessor" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = local.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.probe[0].email}"
}
resource "google_artifact_registry_repository_iam_member" "image_reader" {
  count      = local.enabled ? 1 : 0
  project    = var.project_id
  location   = data.google_artifact_registry_repository.target[0].location
  repository = data.google_artifact_registry_repository.target[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:service-${data.google_project.target[0].number}@serverless-robot-prod.iam.gserviceaccount.com"
}
resource "google_cloud_run_v2_job" "probe" {
  count               = local.enabled ? 1 : 0
  project             = var.project_id
  name                = "espacigo-gcp04-w8-catalog-probe"
  location            = var.region
  deletion_protection = false
  depends_on = [
    google_project_iam_member.sql_client,
    google_secret_manager_secret_iam_member.secret_accessor,
    google_artifact_registry_repository_iam_member.image_reader,
  ]
  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.probe[0].email
      max_retries     = 0
      timeout         = "120s"
      volumes {
        name = "db-password"
        secret {
          secret = local.secret_id
          items {
            version = var.secret_version
            path    = "password"
            mode    = 292
          }
        }
      }
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.target[0].id
          subnetwork = data.google_compute_subnetwork.target[0].id
          tags       = ["espacigo-gcp04-w9-catalog-probe"]
        }
      }
      containers {
        image = var.image_digest
        args  = ["--query-file", "/gcp04_w8_catalog_probe.sql"]
        resources {
          limits = { cpu = "1", memory = "512Mi" }
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = data.google_sql_database_instance.target[0].connection_name
        }
        env {
          name  = "DATABASE_NAME"
          value = "espacigo"
        }
        env {
          name  = "DATABASE_USER"
          value = local.login
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/var/secrets/db/password"
        }
        env {
          name  = "GCP04_W8_EXPECTED_PRIVATE_IP"
          value = "10.189.0.3"
        }
        env {
          name  = "GCP04_W8_PROJECT_ID"
          value = var.project_id
        }
        env {
          name  = "GCP04_W8_SECRET_VERSION"
          value = var.secret_version
        }
        volume_mounts {
          name       = "db-password"
          mount_path = "/var/secrets/db"
        }
      }
    }
  }
  lifecycle {
    precondition {
      condition     = var.confirm_w9_release && var.reviewer == "HernanEspinozaDev" && can(regex("^[a-z0-9-]{1,64}$", var.release_id)) && can(regex("^[0-9a-f]{64}$", var.release_sha256)) && var.project_id == "espacigo-testprod" && var.region == "southamerica-west1" && var.network_name == "espacigo-testprod" && var.subnetwork_name == "espacigo-testprod-serverless" && var.repository_id == "espacigo-testprod" && data.google_sql_database_instance.target[0].connection_name == "espacigo-testprod:southamerica-west1:espacigo-testprod-pg" && data.google_sql_database_instance.target[0].database_version == "POSTGRES_18" && data.google_sql_database_instance.target[0].private_ip_address == "10.189.0.3" && (data.google_secret_manager_secret.target[0].name == "projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password" || data.google_secret_manager_secret.target[0].name == "projects/${data.google_project.target[0].number}/secrets/espacigo-gcp04-w8-catalog-probe-password") && length(data.google_secret_manager_secret.target[0].replication[0].user_managed) == 1 && length(data.google_secret_manager_secret.target[0].replication[0].user_managed[0].replicas) == 1 && data.google_secret_manager_secret.target[0].replication[0].user_managed[0].replicas[0].location == "southamerica-west1" && length(data.google_secret_manager_secret.target[0].replication[0].auto) == 0 && can(regex(local.image_pattern, var.image_digest)) && can(regex("^[1-9][0-9]*$", var.secret_version))
      error_message = "W9 requires an exact approved reviewer release, target, immutable image digest and numeric secret version."
    }
    precondition {
      condition     = var.secret_version != "latest"
      error_message = "A numeric pinned version is required."
    }
  }
}
