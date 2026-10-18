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
  deletion_protection = true
  deletion_policy     = "PREVENT"

  settings {
    edition                          = "ENTERPRISE"
    tier                             = "db-custom-N4-2-4096"
    availability_type                = "ZONAL"
    connector_enforcement            = "REQUIRED"
    data_api_access                  = "DISALLOW_DATA_API"
    deletion_protection_enabled      = true
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
  deletion_policy = "PREVENT"
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

# Private application files use a different bucket from ciphertext receipts and
# transport objects. Infrastructure can be reviewed/applied while the import
# Job remains disabled and before a K_run Secret Manager version exists.
resource "google_storage_bucket" "private_files" {
  count                       = local.import_infrastructure_enabled ? 1 : 0
  project                     = var.project_id
  name                        = "espacigo-gcp03-private-${var.run_id}"
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
    purpose     = "gcp03-private-files"
    run_id      = replace(var.run_id, "-", "_")
  }
}

resource "google_storage_bucket_iam_member" "private_files_creator" {
  # Preserve the existing binding in continuation-plan mode so the plan can
  # change only the recovery Job profile. That entrypoint never constructs the
  # object store in plan mode; copy remains a separately reviewed mode.
  count  = local.private_files_creator_enabled ? 1 : 0
  bucket = google_storage_bucket.private_files[0].name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.admin[0].email}"
  condition {
    title       = "gcp03-private-files-${var.run_id}-run-prefix"
    description = "Allow create-only objects under the exact import run prefix."
    expression  = "resource.name.startsWith('projects/_/buckets/espacigo-gcp03-private-${var.run_id}/objects/runs/${var.run_id}/')"
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
  deletion_protection = true
  deletion_policy     = "PREVENT"
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

resource "google_secret_manager_secret" "transport_key" {
  count               = local.import_infrastructure_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = "${local.prefix}-transport-key"
  deletion_protection = true
  deletion_policy     = "PREVENT"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "transport_key_reader" {
  count     = local.import_infrastructure_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.transport_key[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.admin[0].email}"
}

resource "google_storage_bucket_iam_member" "artifact_reader" {
  count  = local.import_infrastructure_enabled ? 1 : 0
  bucket = google_storage_bucket.artifacts[0].name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.admin[0].email}"
}

resource "google_service_account" "ciphertext_uploader" {
  count        = local.uploader_enabled ? 1 : 0
  project      = var.project_id
  account_id   = "g03o-upload-${var.run_id}"
  display_name = "GCP-03 ciphertext-only uploader ${var.run_id}"
  description  = "Create-only uploader for one run prefix; no secrets or Cloud SQL permissions."
}

resource "google_storage_bucket_iam_member" "ciphertext_uploader" {
  count  = local.uploader_enabled ? 1 : 0
  bucket = google_storage_bucket.artifacts[0].name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.ciphertext_uploader[0].email}"
  condition {
    title       = "gcp03-origin-${var.run_id}-ciphertext-prefix"
    description = "Limit object creation to the single run prefix in the ciphertext bucket."
    expression  = "resource.name.startsWith('projects/_/buckets/espacigo-gcp03-origin-${var.run_id}/objects/runs/${var.run_id}/')"
  }
}

# Temporary, least-privilege human-to-uploader impersonation. This custom
# project role contains only getAccessToken; its binding is on the uploader SA.
resource "google_project_iam_custom_role" "uploader_token_access" {
  count       = local.uploader_impersonation_enabled ? 1 : 0
  project     = var.project_id
  role_id     = "g03oUploadTokenAccessG03o1017"
  title       = "G03O-1017 uploader token access"
  description = "Temporary run-scoped ability to mint access tokens for the ciphertext uploader service account."
  permissions = ["iam.serviceAccounts.getAccessToken"]
}

resource "google_service_account_iam_member" "uploader_token_access" {
  count              = local.uploader_impersonation_enabled ? 1 : 0
  service_account_id = google_service_account.ciphertext_uploader[0].name
  role               = google_project_iam_custom_role.uploader_token_access[0].name
  member             = "user:pasteleriahijitos@gmail.com"
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
