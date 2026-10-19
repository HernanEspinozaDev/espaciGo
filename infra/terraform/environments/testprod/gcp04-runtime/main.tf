locals {
  runtime_enabled   = var.enable_worker || var.enable_migrator || var.enable_bootstrap
  worker_enabled    = var.enable_worker
  migrator_enabled  = var.enable_migrator
  bootstrap_enabled = var.enable_bootstrap
  instance_path     = "projects/${var.project_id}/instances/${var.sql_instance_name}"
}

data "google_compute_network" "existing" {
  count   = local.runtime_enabled ? 1 : 0
  name    = var.network_name
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  count   = local.runtime_enabled ? 1 : 0
  name    = var.subnetwork_name
  region  = var.region
  project = var.project_id
}

data "google_sql_database_instance" "existing" {
  count   = local.runtime_enabled ? 1 : 0
  name    = var.sql_instance_name
  project = var.project_id
}

data "google_artifact_registry_repository" "images" {
  count         = local.runtime_enabled ? 1 : 0
  project       = var.project_id
  location      = var.region
  repository_id = var.artifact_registry_repository_id
}

data "google_project" "current" {
  count      = local.runtime_enabled ? 1 : 0
  project_id = var.project_id
}

resource "google_artifact_registry_repository_iam_member" "cloud_run_reader" {
  count      = local.runtime_enabled ? 1 : 0
  project    = var.project_id
  location   = data.google_artifact_registry_repository.images[0].location
  repository = data.google_artifact_registry_repository.images[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:service-${data.google_project.current[0].number}@serverless-robot-prod.iam.gserviceaccount.com"
}

resource "google_service_account" "privacy_worker" {
  count        = local.worker_enabled ? 1 : 0
  project      = var.project_id
  account_id   = var.worker_service_account_id
  display_name = "EspaciGo GCP-04 privacy cleanup worker"
  description  = "Dedicated continuously running privacy cleanup worker; no API or migration permissions."
}

resource "google_project_iam_member" "worker_cloudsql_client" {
  count   = local.worker_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.privacy_worker[0].email}"

  condition {
    title       = "gcp04-worker-cloudsql-instance"
    description = "Cloud SQL connector access only to the pre-existing EspaciGo instance."
    expression  = "resource.name == '${local.instance_path}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_project_iam_custom_role" "worker_private_files" {
  count       = local.worker_enabled ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoGcp04PrivacyCleanupFiles"
  title       = "EspaciGo GCP-04 privacy cleanup files"
  description = "Read/delete private evidence objects for the dedicated cleanup worker."
  permissions = ["storage.objects.get", "storage.objects.delete"]
}

resource "google_storage_bucket_iam_member" "worker_private_files" {
  count  = local.worker_enabled ? 1 : 0
  bucket = var.private_bucket_name
  role   = google_project_iam_custom_role.worker_private_files[0].name
  member = "serviceAccount:${google_service_account.privacy_worker[0].email}"
}

resource "google_secret_manager_secret_iam_member" "worker_db_password" {
  count     = local.worker_enabled ? 1 : 0
  project   = var.project_id
  secret_id = var.worker_database_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.privacy_worker[0].email}"
}

resource "google_project_iam_member" "migrator_cloudsql_client" {
  count   = local.migrator_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${var.migrator_service_account_email}"

  condition {
    title       = "gcp04-migrator-cloudsql-instance"
    description = "Migration connector access only to the pre-existing EspaciGo instance."
    expression  = "resource.name == '${local.instance_path}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_secret_manager_secret_iam_member" "migrator_db_password" {
  count     = var.manage_migrator_secret_iam && (local.migrator_enabled || local.bootstrap_enabled) ? 1 : 0
  project   = var.project_id
  secret_id = var.migrator_database_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${var.migrator_service_account_email}"
}

resource "google_secret_manager_secret_iam_member" "bootstrap_db_password" {
  count     = local.bootstrap_enabled ? 1 : 0
  project   = var.project_id
  secret_id = var.bootstrap_database_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${var.migrator_service_account_email}"
}

resource "google_secret_manager_secret_iam_member" "api_db_password_for_verify" {
  count     = local.bootstrap_enabled ? 1 : 0
  project   = var.project_id
  secret_id = var.api_database_password_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${var.migrator_service_account_email}"
}

resource "google_cloud_run_v2_worker_pool" "privacy_cleanup" {
  count               = local.worker_enabled ? 1 : 0
  project             = var.project_id
  name                = var.worker_pool_name
  location            = var.region
  launch_stage        = "BETA"
  deletion_protection = true
  depends_on = [
    google_artifact_registry_repository_iam_member.cloud_run_reader,
    google_project_iam_member.worker_cloudsql_client,
    google_storage_bucket_iam_member.worker_private_files,
    google_secret_manager_secret_iam_member.worker_db_password,
  ]

  scaling {
    manual_instance_count = 1
  }

  template {
    volumes {
      name = "database-password"
      secret {
        secret = var.worker_database_password_secret_id
        items {
          version = var.worker_database_password_version
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
        tags       = ["espacigo-gcp04-worker"]
      }
    }
    containers {
      image = var.worker_image
      resources {
        limits = { cpu = "1", memory = "512Mi" }
      }
      env {
        name  = "DATABASE_CONNECTION_MODE"
        value = "cloudsql"
      }
      env {
        name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
        value = data.google_sql_database_instance.existing[0].connection_name
      }
      env {
        name  = "CLOUD_SQL_PRIVATE_IP"
        value = "true"
      }
      env {
        name  = "DATABASE_NAME"
        value = var.database_name
      }
      env {
        name  = "DATABASE_USER"
        value = var.worker_database_user
      }
      env {
        name  = "DATABASE_SET_ROLE"
        value = "espacigo_privacy_cleanup_worker"
      }
      env {
        name  = "BLOB_STORE_BACKEND"
        value = "gcs"
      }
      env {
        name  = "CLOUD_STORAGE_BUCKET"
        value = var.private_bucket_name
      }
      env {
        name  = "CLOUD_STORAGE_OBJECT_PREFIX"
        value = var.private_object_prefix
      }
      env {
        name  = "DATABASE_PASSWORD_FILE"
        value = "/var/secrets/database-password/password"
      }
      volume_mounts {
        name       = "database-password"
        mount_path = "/var/secrets/database-password"
      }
      env {
        name  = "PRIVACY_CLEANUP_INTERVAL"
        value = "10s"
      }
    }
    service_account = google_service_account.privacy_worker[0].email
  }

  lifecycle {
    precondition {
      condition     = var.project_id == "espacigo-testprod" && var.worker_database_user == "espacigo_privacy_worker_login" && var.worker_database_password_secret_id != "" && can(regex("^[1-9][0-9]*$", var.worker_database_password_version)) && can(regex("@sha256:[0-9a-f]{64}$", var.worker_image)) && var.private_bucket_name != ""
      error_message = "Worker activation requires the reviewed project, dedicated worker login, numeric secret version, immutable image digest and existing private bucket."
    }
    precondition {
      condition     = data.google_compute_subnetwork.existing[0].private_ip_google_access && data.google_sql_database_instance.existing[0].region == var.region
      error_message = "Worker requires the existing same-region private subnet and Cloud SQL candidate."
    }
  }
}

resource "google_cloud_run_v2_job" "migrator" {
  count               = local.migrator_enabled ? 1 : 0
  project             = var.project_id
  name                = var.migrator_job_name
  location            = var.region
  deletion_protection = true
  depends_on = [
    google_artifact_registry_repository_iam_member.cloud_run_reader,
    google_project_iam_member.migrator_cloudsql_client,
    google_secret_manager_secret_iam_member.migrator_db_password,
  ]

  template {
    task_count = 1
    template {
      service_account = var.migrator_service_account_email
      max_retries     = 0
      timeout         = "1800s"
      volumes {
        name = "database-password"
        secret {
          secret = var.migrator_database_password_secret_id
          items {
            version = var.migrator_database_password_version
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
          tags       = ["espacigo-gcp04-migrator"]
        }
      }
      containers {
        image = var.migrator_image
        resources { limits = { cpu = "1", memory = "512Mi" } }
        env {
          name  = "DATABASE_URL"
          value = "postgres://${var.migrator_database_user}@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = data.google_sql_database_instance.existing[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        env {
          name  = "DATABASE_NAME"
          value = var.database_name
        }
        env {
          name  = "DATABASE_USER"
          value = var.migrator_database_user
        }
        env {
          name  = "DATABASE_SET_ROLE"
          value = "espacigo_schema_owner"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/var/secrets/database-password/password"
        }
        volume_mounts {
          name       = "database-password"
          mount_path = "/var/secrets/database-password"
        }
      }
    }
  }
  lifecycle {
    precondition {
      condition     = var.project_id == "espacigo-testprod" && can(regex("^[^@]+@[^@]+\\.iam\\.gserviceaccount\\.com$", var.migrator_service_account_email)) && var.migrator_database_user == "espacigo_migrator_login" && var.migrator_database_password_secret_id != "" && can(regex("^[1-9][0-9]*$", var.migrator_database_password_version)) && can(regex("@sha256:[0-9a-f]{64}$", var.migrator_image))
      error_message = "Migrator activation requires the reviewed project, dedicated runtime identity, dedicated DB login, numeric secret version and immutable image digest."
    }
    precondition {
      condition     = data.google_compute_subnetwork.existing[0].private_ip_google_access && data.google_sql_database_instance.existing[0].region == var.region
      error_message = "Migrator requires the existing same-region private subnet and Cloud SQL candidate."
    }
  }
}

resource "google_cloud_run_v2_job" "runtime_bootstrap" {
  count               = local.bootstrap_enabled ? 1 : 0
  project             = var.project_id
  name                = var.bootstrap_job_name
  location            = var.region
  deletion_protection = false
  depends_on = [
    google_artifact_registry_repository_iam_member.cloud_run_reader,
    google_secret_manager_secret_iam_member.bootstrap_db_password,
    google_secret_manager_secret_iam_member.api_db_password_for_verify,
    google_secret_manager_secret_iam_member.migrator_db_password,
  ]

  template {
    task_count = 1
    template {
      service_account = var.migrator_service_account_email
      max_retries     = 0
      timeout         = "300s"
      volumes {
        name = "b1-password"
        secret {
          secret = var.bootstrap_database_password_secret_id
          items {
            version = var.bootstrap_database_password_version
            path    = "password"
            mode    = 256
          }
        }
      }
      volumes {
        name = "api-password"
        secret {
          secret = var.api_database_password_secret_id
          items {
            version = var.api_database_password_version
            path    = "password"
            mode    = 256
          }
        }
      }
      volumes {
        name = "migrator-password"
        secret {
          secret = var.migrator_database_password_secret_id
          items {
            version = var.migrator_database_password_version
            path    = "password"
            mode    = 256
          }
        }
      }
      vpc_access {
        connector = null
        egress    = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].id
          subnetwork = data.google_compute_subnetwork.existing[0].id
          tags       = ["espacigo-gcp04-bootstrap"]
        }
      }
      containers {
        image = var.bootstrap_image
        args  = ["-mode", var.bootstrap_mode]
        resources {
          limits = { cpu = "1", memory = "512Mi" }
        }
        env {
          name  = "GCP04_W7_B1_PASSWORD_FILE"
          value = "/var/secrets/b1/password"
        }
        env {
          name  = "GCP04_W7_API_PASSWORD_FILE"
          value = "/var/secrets/api/password"
        }
        env {
          name  = "GCP04_W7_MIGRATOR_PASSWORD_FILE"
          value = "/var/secrets/migrator/password"
        }
        volume_mounts {
          name       = "b1-password"
          mount_path = "/var/secrets/b1"
        }
        volume_mounts {
          name       = "api-password"
          mount_path = "/var/secrets/api"
        }
        volume_mounts {
          name       = "migrator-password"
          mount_path = "/var/secrets/migrator"
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition     = var.project_id == "espacigo-testprod" && var.region == "southamerica-west1" && var.sql_instance_name == "espacigo-testprod-pg" && var.database_name == "espacigo" && var.network_name == "espacigo-testprod" && var.subnetwork_name == "espacigo-testprod-serverless" && var.migrator_service_account_email == "espacigo-test-migrator@espacigo-testprod.iam.gserviceaccount.com" && var.bootstrap_database_user == "espacigo_gcp04_w7_bootstrap" && var.api_database_user == "espacigo_api_login" && var.migrator_database_user == "espacigo_migrator_login" && var.bootstrap_database_password_secret_id == "espacigo-gcp04-w7-b1-bootstrap-credential" && var.api_database_password_secret_id == "espacigo-test-database-password" && var.migrator_database_password_secret_id == "espacigo-test-database-migrator-password" && var.bootstrap_database_password_version == "1" && var.api_database_password_version == "1" && var.migrator_database_password_version == "1" && can(regex("@sha256:[0-9a-f]{64}$", var.bootstrap_image)) && contains(["bootstrap", "verify-logins"], var.bootstrap_mode)
      error_message = "W7 bootstrap requires fixed database logins, numeric mounted secret versions, a pinned image digest and an explicit supported mode."
    }
    precondition {
      condition     = local.bootstrap_enabled && var.confirm_runtime_database_bootstrap && data.google_compute_subnetwork.existing[0].private_ip_google_access && data.google_sql_database_instance.existing[0].region == var.region
      error_message = "W7 bootstrap requires explicit runtime database bootstrap confirmation and same-region private networking."
    }
  }
}
