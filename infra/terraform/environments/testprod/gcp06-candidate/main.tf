locals {
  enabled           = var.enable_candidate
  protect_candidate = !var.allow_candidate_destroy || var.confirm_candidate_destroy_run_id != var.run_id
  instance_name     = "espacigo-gcp06-${var.run_id}-pg"
  database_name     = "espacigo_gcp06_candidate_${var.run_id}"
  bucket_name       = "espacigo-testprod-gcp06-${var.run_id}"
  object_name       = "gcp06/${var.run_id}/data.sql"
  import_role_id    = "gcp06CandidateObjectGet${replace(var.run_id, "-", "")}"
  upload_role_id    = "gcp06CandidateObjectCreate${replace(var.run_id, "-", "")}"
}

data "google_compute_network" "candidate" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  name    = var.network_name
}

resource "google_sql_database_instance" "candidate" {
  count               = local.enabled ? 1 : 0
  project             = var.project_id
  name                = local.instance_name
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = local.protect_candidate
  deletion_policy     = local.protect_candidate ? "PREVENT" : "DELETE"

  settings {
    edition                     = "ENTERPRISE"
    tier                        = var.instance_tier
    availability_type           = "ZONAL"
    connector_enforcement       = "REQUIRED"
    deletion_protection_enabled = local.protect_candidate
    disk_type                   = "HYPERDISK_BALANCED"
    disk_size                   = var.disk_gib
    disk_autoresize             = false

    user_labels = {
      application = "espacigo"
      environment = "candidate"
      purpose     = "gcp06-v61-v64-transfer"
      run_id      = replace(var.run_id, "-", "_")
      managed_by  = "terraform"
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = data.google_compute_network.candidate[0].self_link
      ssl_mode        = "ENCRYPTED_ONLY"
    }

    backup_configuration {
      enabled                        = false
      point_in_time_recovery_enabled = false
    }
  }
}

resource "google_sql_database" "candidate" {
  count    = local.enabled ? 1 : 0
  project  = var.project_id
  instance = google_sql_database_instance.candidate[0].name
  name     = local.database_name
}

resource "google_storage_bucket" "import" {
  count                       = local.enabled ? 1 : 0
  project                     = var.project_id
  name                        = local.bucket_name
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age            = 7
      matches_prefix = ["gcp06/${var.run_id}/"]
    }
  }
  deletion_policy = local.protect_candidate ? "PREVENT" : "DELETE"

  labels = {
    application = "espacigo"
    purpose     = "gcp06-sanitized-import"
    run_id      = replace(var.run_id, "-", "_")
    managed_by  = "terraform"
  }
}

# Cloud SQL imports the SQL object through its instance service identity. The
# dedicated custom role grants only object get and only on this run-scoped bucket.
resource "google_project_iam_custom_role" "import_object_reader" {
  count       = local.enabled && var.enable_import_binding ? 1 : 0
  project     = var.project_id
  role_id     = local.import_role_id
  title       = "GCP-06 ${var.run_id} candidate import object reader"
  description = "Read sanitized GCP-06 SQL objects from the dedicated run bucket."
  permissions = ["storage.objects.get"]
}

resource "google_storage_bucket_iam_member" "cloud_sql_import_reader" {
  count  = local.enabled && var.enable_import_binding ? 1 : 0
  bucket = google_storage_bucket.import[0].name
  role   = google_project_iam_custom_role.import_object_reader[0].name
  member = "serviceAccount:${google_sql_database_instance.candidate[0].service_account_email_address}"
  condition {
    title       = "gcp06-${var.run_id}-exact-import-object"
    description = "Temporary read access to the single sanitized SQL object for this candidate."
    expression  = "resource.name == 'projects/_/buckets/${local.bucket_name}/objects/${local.object_name}' && request.time < timestamp('${var.import_binding_expires_at}')"
  }
}

resource "google_project_iam_custom_role" "upload_object_creator" {
  count       = local.enabled && var.enable_upload_binding ? 1 : 0
  project     = var.project_id
  role_id     = local.upload_role_id
  title       = "GCP-06 ${var.run_id} candidate upload object creator"
  description = "Create the one sanitized GCP-06 SQL object in its dedicated run bucket."
  permissions = ["storage.objects.create"]
}

resource "google_storage_bucket_iam_member" "uploader" {
  count  = local.enabled && var.enable_upload_binding ? 1 : 0
  bucket = google_storage_bucket.import[0].name
  role   = google_project_iam_custom_role.upload_object_creator[0].name
  member = var.uploader_member
  condition {
    title       = "gcp06-${var.run_id}-exact-upload-object"
    description = "Temporary create-only access to the one sanitized SQL object for this candidate."
    expression  = "resource.name == 'projects/_/buckets/${local.bucket_name}/objects/${local.object_name}' && request.time < timestamp('${var.upload_binding_expires_at}')"
  }
}
