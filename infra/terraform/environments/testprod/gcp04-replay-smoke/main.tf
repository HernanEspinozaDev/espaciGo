locals {
  resources_enabled = var.enable_base
  job_enabled       = var.enable_base && var.enable_job
  protect_base      = !var.allow_base_destroy || var.confirm_base_destroy_run_id != var.run_id
  protect_job       = !var.allow_job_destroy || var.confirm_job_destroy_run_id != var.run_id
  prefix            = "espacigo-gcp04-${var.run_id}"
  runner_sa_email   = "${local.prefix}@${var.project_id}.iam.gserviceaccount.com"
  runner_sa_name    = "projects/${var.project_id}/serviceAccounts/${local.runner_sa_email}"
  bucket_name       = "espacigo-testprod-${var.bucket_suffix}"
  password_secret   = "${local.prefix}-db-owner-password"
  repository_name   = "gcp04-replay-${var.run_id}"
  job_name          = "${local.prefix}-smoke"
  replay_prefix     = "gcp04/${var.run_id}/privacy"
  blob_prefix       = "gcp04/${var.run_id}/evidence"
}

data "google_compute_network" "existing" {
  count   = local.resources_enabled ? 1 : 0
  name    = "espacigo-testprod"
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  count   = local.resources_enabled ? 1 : 0
  name    = "espacigo-testprod-serverless"
  region  = var.region
  project = var.project_id
}

data "google_project" "current" {
  count      = local.resources_enabled ? 1 : 0
  project_id = var.project_id
}

resource "google_service_account" "runner" {
  count           = local.resources_enabled ? 1 : 0
  project         = var.project_id
  account_id      = local.prefix
  display_name    = "GCP-04 replay smoke ${var.run_id}"
  description     = "Run-scoped integration Job only; not an API or worker identity."
  disabled        = false
  deletion_policy = local.protect_base ? "PREVENT" : "DELETE"
}

resource "google_sql_database_instance" "scratch" {
  count               = local.resources_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-pg"
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = local.protect_base
  deletion_policy     = local.protect_base ? "PREVENT" : "DELETE"

  settings {
    edition                     = "ENTERPRISE"
    tier                        = var.instance_tier
    availability_type           = "ZONAL"
    connector_enforcement       = "REQUIRED"
    deletion_protection_enabled = local.protect_base
    disk_type                   = "HYPERDISK_BALANCED"
    disk_size                   = var.disk_gib
    disk_autoresize             = false

    user_labels = {
      application = "espacigo"
      environment = "scratch"
      purpose     = "gcp04-replay-smoke"
      run_id      = replace(var.run_id, "-", "_")
      managed_by  = "terraform"
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = data.google_compute_network.existing[0].self_link
      ssl_mode        = "ENCRYPTED_ONLY"
    }

    backup_configuration {
      enabled                        = false
      point_in_time_recovery_enabled = false
    }
  }

  lifecycle {
    precondition {
      condition     = data.google_compute_subnetwork.existing[0].private_ip_google_access
      error_message = "The existing Direct VPC egress subnet must keep Private Google Access enabled."
    }
    precondition {
      condition     = data.google_compute_subnetwork.existing[0].region == var.region
      error_message = "The Cloud SQL rehearsal and existing private subnet must be in the same region."
    }
  }
}

resource "google_storage_bucket" "private" {
  count                       = local.resources_enabled ? 1 : 0
  project                     = var.project_id
  name                        = local.bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  deletion_policy             = local.protect_base ? "PREVENT" : "DELETE"


  labels = {
    application = "espacigo"
    purpose     = "gcp04-replay-smoke"
    run_id      = replace(var.run_id, "-", "_")
    managed_by  = "terraform"
  }
}

resource "google_secret_manager_secret" "database_owner_password" {
  count               = local.resources_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = local.password_secret
  deletion_protection = local.protect_base
  deletion_policy     = local.protect_base ? "PREVENT" : "DELETE"

  replication {
    auto {}
  }

  # Terraform creates metadata only. Secret versions are provisioned privately.
}

resource "google_artifact_registry_repository" "smoke" {
  count           = local.resources_enabled ? 1 : 0
  project         = var.project_id
  location        = var.region
  repository_id   = local.repository_name
  description     = "Digest-pinned GCP-04 replay smoke image for ${var.run_id}."
  format          = "DOCKER"
  mode            = "STANDARD_REPOSITORY"
  deletion_policy = local.protect_base ? "PREVENT" : "DELETE"
}

# The runtime identity is scoped to the single disposable Cloud SQL instance.
resource "google_project_iam_member" "runner_cloudsql_client" {
  count   = local.resources_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${local.runner_sa_email}"

  condition {
    title       = "gcp04-${var.run_id}-instance"
    description = "Connector access only to this run-scoped Cloud SQL instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_storage_bucket_iam_member" "runner_objects" {
  count  = local.resources_enabled ? 1 : 0
  bucket = google_storage_bucket.private[0].name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${local.runner_sa_email}"
}

resource "google_secret_manager_secret_iam_member" "runner_database_password" {
  count     = local.resources_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.database_owner_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.runner_sa_email}"
}

# Cloud Run's service agent pulls the image; the job identity gets no registry role.
resource "google_artifact_registry_repository_iam_member" "cloud_run_reader" {
  count      = local.resources_enabled ? 1 : 0
  project    = var.project_id
  location   = google_artifact_registry_repository.smoke[0].location
  repository = google_artifact_registry_repository.smoke[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:service-${data.google_project.current[0].number}@serverless-robot-prod.iam.gserviceaccount.com"
}

resource "google_cloud_run_v2_job" "smoke" {
  count               = local.job_enabled ? 1 : 0
  project             = var.project_id
  name                = local.job_name
  location            = var.region
  deletion_protection = local.protect_job

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = local.runner_sa_email
      timeout         = "900s"
      max_retries     = 0

      containers {
        image   = var.runner_image
        command = ["/gcp04-replay-smoke"]
        args    = ["-test.run=^TestGCP04ReplayTwoProcessesPostgres18$", "-test.timeout=120s"]

        env {
          name  = "RUN_GCP04_REPLAY_INTEGRATION"
          value = "YES"
        }
        env {
          name  = "GCP04_REPLAY_CLOUD"
          value = "YES"
        }
        env {
          name  = "GCP04_REPLAY_DATABASE_PASSWORD_FILE"
          value = "/secrets/database-owner/password"
        }
        env {
          name  = "GCP04_REPLAY_CLOUD_SQL_INSTANCE"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "GCP04_REPLAY_CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        env {
          name  = "GCP04_REPLAY_BUCKET"
          value = google_storage_bucket.private[0].name
        }
        env {
          name  = "GCP04_REPLAY_PREFIX"
          value = local.replay_prefix
        }
        env {
          name  = "GCP04_REPLAY_BLOB_PREFIX"
          value = local.blob_prefix
        }
        env {
          name  = "TEST_DATABASE_URL"
          value = "postgres://postgres@localhost:5432/postgres?sslmode=disable"
        }

        volume_mounts {
          name       = "database-owner-password"
          mount_path = "/secrets/database-owner"
        }

        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = "database-owner-password"
        secret {
          secret = google_secret_manager_secret.database_owner_password[0].secret_id
          items {
            version = var.database_owner_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition     = var.confirm_image_digest && can(regex("^[1-9][0-9]*$", var.database_owner_secret_version))
      error_message = "The smoke Job requires an independently verified image and active numeric password version."
    }
  }
}

resource "google_cloud_run_v2_job_iam_member" "operator" {
  count    = local.job_enabled ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_job.smoke[0].location
  name     = google_cloud_run_v2_job.smoke[0].name
  role     = "roles/run.invoker"
  member   = var.operator_member
}
