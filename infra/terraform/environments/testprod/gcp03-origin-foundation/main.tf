data "google_compute_network" "existing" {
  count   = local.foundation_enabled ? 1 : 0
  name    = "espacigo-testprod"
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  count   = local.foundation_enabled ? 1 : 0
  name    = "espacigo-testprod-serverless"
  region  = var.region
  project = var.project_id
}

resource "google_sql_database_instance" "origin" {
  count               = local.foundation_enabled ? 1 : 0
  project             = var.project_id
  name                = local.instance_name
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = !var.allow_origin_destroy
  deletion_policy     = var.allow_origin_destroy ? "DELETE" : "PREVENT"

  settings {
    edition                          = "ENTERPRISE"
    tier                             = "db-custom-N4-2-4096"
    availability_type                = "ZONAL"
    connector_enforcement            = "REQUIRED"
    data_api_access                  = "DISALLOW_DATA_API"
    deletion_protection_enabled      = !var.allow_origin_destroy
    disk_type                        = "HYPERDISK_BALANCED"
    disk_size                        = 20
    disk_autoresize                  = false
    data_disk_provisioned_iops       = 4000
    data_disk_provisioned_throughput = 170
    user_labels = {
      application = "espacigo"
      environment = "testprod"
      purpose     = "gcp03-origin"
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
    precondition {
      condition     = data.google_compute_subnetwork.existing[0].private_ip_google_access && data.google_compute_subnetwork.existing[0].region == var.region
      error_message = "The existing serverless subnet must have Private Google Access and match the SQL region."
    }
  }
}

resource "google_sql_database" "origin" {
  count           = local.foundation_enabled ? 1 : 0
  project         = var.project_id
  name            = "espacigo_scratch"
  instance        = google_sql_database_instance.origin[0].name
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = var.allow_origin_destroy ? "DELETE" : "PREVENT"
}

resource "google_storage_bucket" "artifacts" {
  count                       = local.foundation_enabled ? 1 : 0
  project                     = var.project_id
  name                        = local.prefix
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  soft_delete_policy {
    retention_duration_seconds = 0
  }
  labels = {
    application = "espacigo"
    purpose     = "gcp03-origin"
    run_id      = replace(var.run_id, "-", "_")
  }
}

resource "google_service_account" "admin" {
  count        = local.foundation_enabled ? 1 : 0
  project      = var.project_id
  account_id   = local.admin_account_id
  display_name = "GCP-03 isolated origin bootstrap admin ${var.run_id}"
}

resource "google_secret_manager_secret" "admin_password" {
  count               = local.foundation_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = "${local.prefix}-postgres-password"
  deletion_protection = !var.allow_origin_destroy
  deletion_policy     = var.allow_origin_destroy ? "DELETE" : "PREVENT"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "admin_password_reader" {
  count     = local.foundation_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.admin_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.admin[0].email}"
}

resource "google_project_iam_member" "admin_sql_client" {
  count   = local.foundation_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.admin[0].email}"
  condition {
    title       = "gcp03-origin-${var.run_id}-admin-instance"
    description = "Restrict connector access to the one run-scoped Cloud SQL instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.instance_name}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}
