resource "google_sql_database_instance" "restore" {
  count               = local.foundation_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-pg"
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = !var.allow_restore_destroy
  deletion_policy     = var.allow_restore_destroy ? "DELETE" : "PREVENT"

  settings {
    edition                     = "ENTERPRISE"
    tier                        = "db-custom-N4-2-4096"
    availability_type           = "ZONAL"
    connector_enforcement       = "REQUIRED"
    deletion_protection_enabled = !var.allow_restore_destroy
    disk_type                   = "HYPERDISK_BALANCED"
    disk_size                   = 20
    disk_autoresize             = false
    user_labels = {
      application = "espacigo"
      environment = "testprod"
      purpose     = "gcp03-restore"
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
    database_flags {
      name  = "cloudsql.iam_authentication"
      value = "on"
    }
  }
  lifecycle {
    prevent_destroy = false
    precondition {
      condition     = data.google_compute_subnetwork.existing[0].private_ip_google_access && data.google_compute_subnetwork.existing[0].region == var.region
      error_message = "The existing serverless subnet must have Private Google Access and match the SQL region."
    }
  }
}

resource "google_sql_database" "restore" {
  count           = local.foundation_enabled ? 1 : 0
  project         = var.project_id
  name            = "espacigo_scratch"
  instance        = google_sql_database_instance.restore[0].name
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = var.allow_restore_destroy ? "DELETE" : "PREVENT"
}

resource "google_storage_bucket" "artifacts" {
  count                       = local.foundation_enabled ? 1 : 0
  project                     = var.project_id
  name                        = local.prefix
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  soft_delete_policy {
    retention_duration_seconds = 0
  }
  lifecycle_rule {
    condition {
      age = 3
    }
    action {
      type = "Delete"
    }
  }
  labels = { application = "espacigo", purpose = "gcp03-restore", run_id = replace(var.run_id, "-", "_") }
}

resource "google_service_account" "admin" {
  count        = local.foundation_enabled ? 1 : 0
  project      = var.project_id
  account_id   = "g03-restore-admin-${var.run_id}"
  display_name = "GCP-03 synthetic restore bootstrap and retirement ${var.run_id}"
}

resource "google_secret_manager_secret" "postgres_password" {
  count               = local.foundation_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = "${local.prefix}-postgres-password"
  deletion_protection = !var.allow_restore_destroy
  deletion_policy     = var.allow_restore_destroy ? "DELETE" : "PREVENT"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "admin_password_reader" {
  count     = local.foundation_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.postgres_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.admin[0].email}"
}

resource "google_project_iam_member" "admin_sql_client" {
  count   = local.foundation_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.admin[0].email}"
  condition {
    title       = "gcp03-restore-${var.run_id}-admin-instance"
    description = "Restrict temporary administrative connector access to the one run-scoped instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_service_account" "verify" {
  count        = local.restore_enabled ? 1 : 0
  project      = var.project_id
  account_id   = "g03-verify-${var.run_id}"
  display_name = "GCP-03 least-privilege synthetic restore verifier ${var.run_id}"
}

resource "google_project_iam_member" "verify_sql_client" {
  count   = local.restore_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.verify[0].email}"
  condition {
    title       = "gcp03-restore-${var.run_id}-verify-instance"
    description = "Restrict verifier connector access to the one run-scoped instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_project_iam_member" "verify_sql_instance_user" {
  count   = local.restore_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.instanceUser"
  member  = "serviceAccount:${google_service_account.verify[0].email}"
  condition {
    title       = "gcp03-restore-${var.run_id}-verify-login"
    description = "Restrict temporary verifier IAM database login capability to the run-scoped instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_sql_user" "verify_iam" {
  count           = local.restore_enabled ? 1 : 0
  project         = var.project_id
  instance        = google_sql_database_instance.restore[0].name
  name            = "${google_service_account.verify[0].account_id}@${var.project_id}.iam"
  type            = "CLOUD_IAM_SERVICE_ACCOUNT"
  database_roles  = [local.verifier_role]
  deletion_policy = (var.allow_restore_destroy || var.allow_restore_iam_user_destroy) ? "DELETE" : "PREVENT"
}

resource "google_service_account" "restore" {
  count        = local.restore_enabled ? 1 : 0
  project      = var.project_id
  account_id   = "g03-restore-${var.run_id}"
  display_name = "GCP-03 one-shot synthetic restore ${var.run_id}"
}

resource "google_project_iam_member" "sql_client" {
  count   = local.restore_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.restore[0].email}"
  condition {
    title       = "gcp03-restore-${var.run_id}-instance"
    description = "Restrict connector access to the one run-scoped restore instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_project_iam_member" "sql_instance_user" {
  count   = local.restore_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.instanceUser"
  member  = "serviceAccount:${google_service_account.restore[0].email}"
  condition {
    title       = "gcp03-restore-${var.run_id}-instance-login"
    description = "Restrict temporary IAM database login capability to the run-scoped restore instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_storage_bucket_iam_member" "artifact_reader" {
  count  = local.restore_enabled ? 1 : 0
  bucket = google_storage_bucket.artifacts[0].name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.restore[0].email}"
}

resource "google_storage_bucket_iam_member" "diagnostic_artifact_reader" {
  count  = local.diagnostic_enabled ? 1 : 0
  bucket = google_storage_bucket.artifacts[0].name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.verify[0].email}"
}

resource "google_sql_user" "restore_iam" {
  count           = local.restore_enabled ? 1 : 0
  project         = var.project_id
  instance        = google_sql_database_instance.restore[0].name
  name            = "${google_service_account.restore[0].account_id}@${var.project_id}.iam"
  type            = "CLOUD_IAM_SERVICE_ACCOUNT"
  database_roles  = [local.owner_role]
  deletion_policy = (var.allow_restore_destroy || var.allow_restore_iam_user_destroy) ? "DELETE" : "PREVENT"
}

resource "google_cloud_run_v2_job" "restore" {
  count               = local.restore_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-restore"
  location            = var.region
  deletion_protection = !var.allow_restore_destroy
  depends_on          = [google_service_account_iam_member.restore_operator_act_as]

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.restore_service_account_email
      timeout         = "720s"
      max_retries     = 0
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
      containers {
        name  = "restore"
        image = local.restore_phase_image
        env {
          name  = "GCP03_PHASE"
          value = "restore"
        }
        env {
          name  = "GCP03_PROJECT"
          value = var.project_id
        }
        env {
          name  = "GCP03_REGION"
          value = var.region
        }
        env {
          name  = "GCP03_INSTANCE"
          value = google_sql_database_instance.restore[0].name
        }
        env {
          name  = "GCP03_RUN_ID"
          value = var.run_id
        }
        env {
          name  = "GCP03_BUCKET"
          value = google_storage_bucket.artifacts[0].name
        }
        env {
          name  = "GCP03_MANIFEST_SHA256"
          value = var.manifest_sha256
        }
        env {
          name  = "GCP03_MIGRATION_SHA256"
          value = var.migration_inventory_sha256
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.restore[0].connection_name}/espacigo_scratch"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://g03-restore-${var.run_id}%40${var.project_id}.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
        }
      }
      containers {
        name    = "cloud-sql-proxy"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--auto-iam-authn", "--address=127.0.0.1", "--port=5432", "--admin-port=9091", "--quitquitquit", "--exit-zero-on-sigterm", google_sql_database_instance.restore[0].connection_name]
      }
    }
  }
}

resource "google_cloud_run_v2_job" "verify" {
  count               = local.restore_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-verify"
  location            = var.region
  deletion_protection = !var.allow_restore_destroy
  depends_on          = [google_service_account_iam_member.verify_operator_act_as]
  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.verify_service_account_email
      timeout         = "720s"
      max_retries     = 0
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
      containers {
        name  = "verify"
        image = var.executor_image
        env {
          name  = "GCP03_PHASE"
          value = "verify"
        }
        env {
          name  = "GCP03_PROJECT"
          value = var.project_id
        }
        env {
          name  = "GCP03_REGION"
          value = var.region
        }
        env {
          name  = "GCP03_INSTANCE"
          value = google_sql_database_instance.restore[0].name
        }
        env {
          name  = "GCP03_RUN_ID"
          value = var.run_id
        }
        env {
          name  = "GCP03_MANIFEST_SHA256"
          value = var.manifest_sha256
        }
        env {
          name  = "GCP03_MIGRATION_SHA256"
          value = var.migration_inventory_sha256
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.restore[0].connection_name}/espacigo_scratch"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://g03-verify-${var.run_id}%40${var.project_id}.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
        }
      }
      containers {
        name    = "cloud-sql-proxy"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--auto-iam-authn", "--address=127.0.0.1", "--port=5432", "--admin-port=9091", "--quitquitquit", "--exit-zero-on-sigterm", google_sql_database_instance.restore[0].connection_name]
      }
    }
  }
}

resource "google_cloud_run_v2_job" "diagnose" {
  count               = local.diagnostic_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-diagnose"
  location            = var.region
  deletion_protection = !var.allow_restore_destroy
  depends_on          = [google_service_account_iam_member.verify_operator_act_as, google_storage_bucket_iam_member.diagnostic_artifact_reader]
  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.verify_service_account_email
      timeout         = "300s"
      max_retries     = 0
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
      containers {
        name  = "diagnose"
        image = var.diagnostic_image
        env {
          name  = "GCP03_PHASE"
          value = "diagnose"
        }
        env {
          name  = "GCP03_PROJECT"
          value = var.project_id
        }
        env {
          name  = "GCP03_REGION"
          value = var.region
        }
        env {
          name  = "GCP03_INSTANCE"
          value = google_sql_database_instance.restore[0].name
        }
        env {
          name  = "GCP03_RUN_ID"
          value = var.run_id
        }
        env {
          name  = "GCP03_BUCKET"
          value = google_storage_bucket.artifacts[0].name
        }
        env {
          name  = "GCP03_MANIFEST_SHA256"
          value = var.manifest_sha256
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.restore[0].connection_name}/espacigo_scratch"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://g03-verify-${var.run_id}%40${var.project_id}.iam@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
        }
      }
      containers {
        name    = "cloud-sql-proxy"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--auto-iam-authn", "--address=127.0.0.1", "--port=5432", "--admin-port=9091", "--quitquitquit", "--exit-zero-on-sigterm", google_sql_database_instance.restore[0].connection_name]
      }
    }
  }
}

resource "google_cloud_run_v2_job" "admin_phase" {
  for_each            = local.admin_jobs
  project             = var.project_id
  name                = "${local.prefix}-${each.key}"
  location            = var.region
  deletion_protection = !var.allow_restore_destroy
  depends_on          = [google_service_account_iam_member.bootstrap_deployer_act_as]
  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.admin_service_account_email
      timeout         = "720s"
      max_retries     = 0
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
      containers {
        name  = each.key
        image = var.executor_image
        env {
          name  = "GCP03_PHASE"
          value = each.value
        }
        env {
          name  = "GCP03_PROJECT"
          value = var.project_id
        }
        env {
          name  = "GCP03_REGION"
          value = var.region
        }
        env {
          name  = "GCP03_INSTANCE"
          value = google_sql_database_instance.restore[0].name
        }
        env {
          name  = "GCP03_RUN_ID"
          value = var.run_id
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.restore[0].connection_name}/espacigo_scratch"
        }
        env {
          name  = "GCP03_ADMIN_PASSWORD_FILE"
          value = "/secrets/postgres/password"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://postgres@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
        }
        volume_mounts {
          name       = "admin-password"
          mount_path = "/secrets/postgres"
        }
      }
      containers {
        name    = "cloud-sql-proxy"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--address=127.0.0.1", "--port=5432", "--admin-port=9091", "--quitquitquit", "--exit-zero-on-sigterm", google_sql_database_instance.restore[0].connection_name]
      }
      volumes {
        name = "admin-password"
        secret {
          secret = google_secret_manager_secret.postgres_password[0].secret_id
          items {
            version = var.admin_password_secret_version
            path    = "password"
          }
        }
      }
    }
  }
}

resource "google_cloud_run_v2_job" "retirement_diagnose" {
  count               = local.retirement_diagnostic_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.prefix}-retire-diagnose"
  location            = var.region
  deletion_protection = !var.allow_restore_destroy
  depends_on          = [google_service_account_iam_member.bootstrap_deployer_act_as]
  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = local.admin_service_account_email
      timeout         = "300s"
      max_retries     = 0
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
      containers {
        name  = "retire-diagnose"
        image = var.retirement_diagnostic_image
        env {
          name  = "GCP03_PHASE"
          value = "retire-diagnose"
        }
        env {
          name  = "GCP03_PROJECT"
          value = var.project_id
        }
        env {
          name  = "GCP03_REGION"
          value = var.region
        }
        env {
          name  = "GCP03_INSTANCE"
          value = google_sql_database_instance.restore[0].name
        }
        env {
          name  = "GCP03_RUN_ID"
          value = var.run_id
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.restore[0].connection_name}/espacigo_scratch"
        }
        env {
          name  = "GCP03_ADMIN_PASSWORD_FILE"
          value = "/secrets/postgres/password"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://postgres@127.0.0.1:5432/espacigo_scratch?sslmode=disable"
        }
        volume_mounts {
          name       = "admin-password"
          mount_path = "/secrets/postgres"
        }
      }
      containers {
        name    = "cloud-sql-proxy"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--address=127.0.0.1", "--port=5432", "--admin-port=9091", "--quitquitquit", "--exit-zero-on-sigterm", google_sql_database_instance.restore[0].connection_name]
      }
      volumes {
        name = "admin-password"
        secret {
          secret = google_secret_manager_secret.postgres_password[0].secret_id
          items {
            version = var.admin_password_secret_version
            path    = "password"
          }
        }
      }
    }
  }
}

resource "google_service_account_iam_member" "bootstrap_deployer_act_as" {
  count              = local.bootstrap_job_enabled ? 1 : 0
  service_account_id = google_service_account.admin[0].name
  role               = "roles/iam.serviceAccountUser"
  member             = var.bootstrap_operator_member
}

resource "google_cloud_run_v2_job_iam_member" "bootstrap_invoker" {
  count    = local.bootstrap_job_enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.admin_phase["bootstrap"].name
  role     = "roles/run.invoker"
  member   = var.bootstrap_operator_member
}

resource "google_service_account_iam_member" "restore_operator_act_as" {
  count              = local.restore_enabled ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.restore_service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.bootstrap_operator_member
  depends_on         = [google_service_account.restore]
}

resource "google_service_account_iam_member" "verify_operator_act_as" {
  count              = local.restore_enabled ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${local.verify_service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.bootstrap_operator_member
  depends_on         = [google_service_account.verify]
}

resource "google_cloud_run_v2_job_iam_member" "restore_invoker" {
  count    = local.restore_enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.restore[0].name
  role     = "roles/run.invoker"
  member   = var.bootstrap_operator_member
}

resource "google_cloud_run_v2_job_iam_member" "verify_invoker" {
  count    = local.restore_enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.verify[0].name
  role     = "roles/run.invoker"
  member   = var.bootstrap_operator_member
}

resource "google_cloud_run_v2_job_iam_member" "diagnostic_invoker" {
  count    = local.diagnostic_enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.diagnose[0].name
  role     = "roles/run.invoker"
  member   = var.bootstrap_operator_member
}

resource "google_cloud_run_v2_job_iam_member" "retire_invoker" {
  count    = local.restore_enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.admin_phase["retire"].name
  role     = "roles/run.invoker"
  member   = var.bootstrap_operator_member
}

resource "google_cloud_run_v2_job_iam_member" "retirement_diagnostic_invoker" {
  count    = local.retirement_diagnostic_enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.retirement_diagnose[0].name
  role     = "roles/run.invoker"
  member   = var.bootstrap_operator_member
}
