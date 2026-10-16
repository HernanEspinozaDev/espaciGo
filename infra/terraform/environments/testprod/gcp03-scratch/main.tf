locals {
  name_prefix     = "espacigo-gcp03-${var.run_id}"
  scratch_enabled = var.enable_scratch
  job_enabled     = var.enable_scratch && var.enable_job
  protect_scratch = !var.allow_scratch_destroy
  job_phases = {
    bootstrap = "bootstrap"
    migrate   = "migrate"
    finalize  = "finalize"
    retire    = "retire"
  }
}

data "google_compute_network" "existing" {
  name    = var.network_name
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  name    = var.subnetwork_name
  region  = var.region
  project = var.project_id
}

data "google_project" "current" {
  project_id = var.project_id
}

resource "google_service_account" "scratch_migrator" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  account_id      = "gcp03-scratch-${var.run_id}"
  display_name    = "GCP-03 disposable migration runner ${var.run_id}"
  description     = "Scratch only. No access to persistent database or application secrets."
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

resource "google_project_iam_member" "scratch_cloudsql_client" {
  count   = local.scratch_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.scratch_migrator[0].email}"
}

resource "google_secret_manager_secret" "scratch_password" {
  count               = local.scratch_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = "${local.name_prefix}-migrator-password"
  deletion_protection = local.protect_scratch
  deletion_policy     = local.protect_scratch ? "PREVENT" : "DELETE"

  replication {
    auto {}
  }

  # Terraform creates metadata only; no secret version or value is managed here.
}

resource "google_secret_manager_secret_iam_member" "scratch_password_access" {
  count     = local.scratch_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.scratch_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.scratch_migrator[0].email}"
}

resource "google_artifact_registry_repository" "scratch" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  location        = var.region
  repository_id   = "gcp03-scratch-${var.run_id}"
  description     = "Digest-pinned disposable GCP-03 migrator image ${var.run_id}."
  format          = "DOCKER"
  mode            = "STANDARD_REPOSITORY"
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

# Cloud Run's service agent pulls the image; the runtime identity receives no registry role.
resource "google_artifact_registry_repository_iam_member" "cloud_run_reader" {
  count      = local.scratch_enabled ? 1 : 0
  project    = var.project_id
  location   = google_artifact_registry_repository.scratch[0].location
  repository = google_artifact_registry_repository.scratch[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:service-${data.google_project.current.number}@serverless-robot-prod.iam.gserviceaccount.com"
}

resource "google_sql_database_instance" "scratch" {
  count               = local.scratch_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.name_prefix}-pg"
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = local.protect_scratch
  deletion_policy     = local.protect_scratch ? "PREVENT" : "DELETE"

  settings {
    edition                     = "ENTERPRISE"
    tier                        = var.instance_tier
    availability_type           = "ZONAL"
    connector_enforcement       = "REQUIRED"
    deletion_protection_enabled = local.protect_scratch
    disk_type                   = "HYPERDISK_BALANCED"
    disk_size                   = 20
    disk_autoresize             = false
    user_labels = {
      application = "espacigo"
      environment = "scratch"
      purpose     = "gcp03-compatibility"
      run_id      = replace(var.run_id, "-", "_")
      managed_by  = "terraform"
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = data.google_compute_network.existing.self_link
      ssl_mode        = "ENCRYPTED_ONLY"
    }

    backup_configuration {
      enabled                        = false
      point_in_time_recovery_enabled = false
    }
  }

  lifecycle {
    prevent_destroy = false
    precondition {
      condition     = data.google_compute_subnetwork.existing.private_ip_google_access
      error_message = "Existing subnet must have Private Google Access enabled before a future scratch plan/apply."
    }
    precondition {
      condition     = var.region == data.google_compute_subnetwork.existing.region
      error_message = "Scratch Cloud SQL and Direct VPC subnet must use the same region."
    }
  }
}

resource "google_sql_database" "scratch" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  name            = var.database_name
  instance        = google_sql_database_instance.scratch[0].name
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

resource "google_cloud_run_v2_job" "migrator" {
  for_each            = local.job_enabled ? local.job_phases : {}
  project             = var.project_id
  name                = "${local.name_prefix}-${each.key}"
  location            = var.region
  deletion_protection = !var.allow_scratch_destroy

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = google_service_account.scratch_migrator[0].email
      timeout         = "14400s"
      max_retries     = 0

      containers {
        image = var.migrator_image
        env {
          name  = "GCP03_PHASE"
          value = each.value
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://${var.database_user}@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/migrator/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        env {
          name  = "DBMIGRATE_TIMEOUT"
          value = "4h"
        }
        volume_mounts {
          name       = "scratch-migrator-password"
          mount_path = "/secrets/migrator"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = "scratch-migrator-password"
        secret {
          secret = google_secret_manager_secret.scratch_password[0].secret_id
          items {
            version = "latest"
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition     = local.scratch_enabled && var.confirm_image_pushed && var.confirm_scratch_secret_version
      error_message = "The one-shot Job must remain gated until its reviewed image digest exists and the temporary secret has an active version."
    }
  }
}

resource "google_cloud_run_v2_job_iam_member" "invoke_operator" {
  for_each = google_cloud_run_v2_job.migrator
  project  = var.project_id
  location = each.value.location
  name     = each.value.name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_service_account_iam_member" "operator_act_as" {
  count              = local.job_enabled ? 1 : 0
  service_account_id = google_service_account.scratch_migrator[0].name
  role               = "roles/iam.serviceAccountUser"
  member             = var.operator_member
}
