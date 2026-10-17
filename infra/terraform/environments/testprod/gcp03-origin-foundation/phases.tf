resource "google_cloud_run_v2_job" "origin_bootstrap" {
  count               = local.admin_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-bootstrap"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = google_service_account.admin[0].email
      timeout         = "1800s"
      max_retries     = 0

      containers {
        image = var.origin_executor_image
        env {
          name  = "GCP03_ORIGIN_PHASE"
          value = "bootstrap"
        }
        env {
          name  = "GCP03_ORIGIN_CONFIRM_TARGET"
          value = "${google_sql_database_instance.origin[0].connection_name}/${google_sql_database.origin[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://postgres@localhost:5432/espacigo_scratch?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/admin/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.origin[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }

        volume_mounts {
          name       = "origin-admin-password"
          mount_path = "/secrets/admin"
        }
        resources {
          limits = {
            cpu    = "1"
            memory = "1Gi"
          }
        }
      }

      volumes {
        name = "origin-admin-password"
        secret {
          secret       = google_secret_manager_secret.admin_password[0].secret_id
          default_mode = 292
          items {
            version = var.admin_password_secret_version
            path    = "password"
            mode    = 292
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
      condition     = local.admin_enabled && var.confirm_admin_password_secret_version && can(regex("^[1-9][0-9]*$", var.admin_password_secret_version))
      error_message = "Bootstrap Job requires an exact active numeric admin secret version and explicit confirmation."
    }
  }
}

resource "google_cloud_run_v2_job" "origin_import" {
  count               = local.import_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-import"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = google_service_account.admin[0].email
      timeout         = "14400s"
      max_retries     = 0

      containers {
        name       = "origin-import"
        image      = var.origin_import_image
        depends_on = ["cloud-sql-proxy"]
        env {
          name  = "GCP03_ORIGIN_IMPORT_BUCKET"
          value = local.prefix
        }
        env {
          name  = "GCP03_ORIGIN_PRIVATE_FILES_BUCKET"
          value = google_storage_bucket.private_files[0].name
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT"
          value = var.import_receipt_object
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION"
          value = var.import_receipt_generation
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_SHA256"
          value = var.import_receipt_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_BYTES"
          value = tostring(var.import_receipt_bytes)
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_MANIFEST_SHA256"
          value = var.import_manifest_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_BOOTSTRAP_SHA256"
          value = var.bootstrap_evidence_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_OBSERVER_SHA256"
          value = var.import_observer_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_NORMALIZATION_SHA256"
          value = var.import_normalization_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_CONFIRM_TARGET"
          value = "${google_sql_database_instance.origin[0].connection_name}/espacigo_scratch"
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE"
          value = "/secrets/transport/k_run"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://postgres@localhost:5432/espacigo_scratch?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/admin/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.origin[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        volume_mounts {
          name       = "origin-admin-password"
          mount_path = "/secrets/admin"
        }
        volume_mounts {
          name       = "origin-transport-key"
          mount_path = "/secrets/transport"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      containers {
        name    = "cloud-sql-proxy"
        image   = var.origin_proxy_image
        command = ["/cloud-sql-proxy"]
        args = [
          "--private-ip",
          "--address=127.0.0.1",
          "--port=5432",
          "--admin-port=9091",
          "--quitquitquit",
          "--exit-zero-on-sigterm",
          "--health-check",
          "--http-address=0.0.0.0",
          "--http-port=9801",
          google_sql_database_instance.origin[0].connection_name,
        ]
        startup_probe {
          http_get {
            path = "/startup"
            port = 9801
          }
          period_seconds    = 1
          timeout_seconds   = 1
          failure_threshold = 60
        }
        resources {
          limits = {
            cpu    = "1"
            memory = "256Mi"
          }
        }
      }

      volumes {
        name = "origin-admin-password"
        secret {
          secret       = google_secret_manager_secret.admin_password[0].secret_id
          default_mode = 292
          items {
            version = var.admin_password_secret_version
            path    = "password"
            mode    = 292
          }
        }
      }
      volumes {
        name = "origin-transport-key"
        secret {
          secret       = google_secret_manager_secret.transport_key[0].secret_id
          default_mode = 292
          items {
            version = var.transport_key_secret_version
            path    = "k_run"
            mode    = 292
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
      condition     = local.import_enabled && var.confirm_transport_key_secret_version && var.confirm_admin_password_secret_version && var.confirm_origin_bootstrap_completed && var.import_receipt_object == "runs/${var.run_id}/receipt.json"
      error_message = "Import Job requires the exact run receipt, reviewed evidence, and numeric secret versions."
    }
  }
}

# The observer uses its own IAM database identity and connector permissions.
# It is intentionally not granted Secret Manager or Cloud Storage access.
resource "google_service_account" "origin_observer" {
  count        = local.observer_enabled ? 1 : 0
  project      = var.project_id
  account_id   = local.observer_account_id
  display_name = "GCP-03 origin pre-restore observer ${var.run_id}"
  description  = "Fixed read-only metadata observer; no secret or bucket access."
}

resource "google_project_iam_member" "origin_observer_cloudsql_client" {
  count   = local.observer_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.origin_observer[0].email}"

  condition {
    title       = "gcp03-origin-${var.run_id}-observer-connector"
    description = "Limit the observer connector to the one run-scoped Cloud SQL instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.instance_name}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_project_iam_member" "origin_observer_instance_user" {
  count   = local.observer_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.instanceUser"
  member  = "serviceAccount:${google_service_account.origin_observer[0].email}"

  condition {
    title       = "gcp03-origin-${var.run_id}-observer-login"
    description = "Limit IAM database login to the one run-scoped Cloud SQL instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.instance_name}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_sql_user" "origin_observer" {
  count    = local.observer_enabled ? 1 : 0
  project  = var.project_id
  instance = google_sql_database_instance.origin[0].name
  # Derive the Cloud SQL IAM login from the account resource itself. Besides
  # keeping the principal exact, this graph edge makes Cloud SQL wait until the
  # service account exists before registering its IAM database user.
  name           = trimsuffix(google_service_account.origin_observer[0].email, ".gserviceaccount.com")
  type           = "CLOUD_IAM_SERVICE_ACCOUNT"
  database_roles = ["espacigo_gcp03_observer"]
  lifecycle {
    precondition {
      condition     = var.confirm_origin_bootstrap_completed && can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256))
      error_message = "Create the IAM observer database user only after hash-bound bootstrap evidence confirms its group role exists."
    }
  }
}

resource "google_cloud_run_v2_job" "origin_observer" {
  count               = local.observer_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-observe"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = google_service_account.origin_observer[0].email
      timeout         = "1800s"
      max_retries     = 0

      containers {
        image = var.origin_executor_image
        env {
          name  = "GCP03_ORIGIN_PHASE"
          value = "observe"
        }
        env {
          name  = "GCP03_ORIGIN_CONFIRM_TARGET"
          value = "${google_sql_database_instance.origin[0].connection_name}/${google_sql_database.origin[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = local.observer_database_url
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.origin[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }

        resources {
          limits = {
            cpu    = "1"
            memory = "1Gi"
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

  depends_on = [google_sql_user.origin_observer]
}
