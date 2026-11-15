# Canonical media bucket/IAM addresses: preserve the already-applied *_only
# state addresses. Do not add moved/import blocks or duplicate media resources.
# Data-class enablement also activates these addresses, alongside contracts/KYC.
resource "google_storage_bucket" "profile_space_media_only" {
  count                       = (var.enable_profile_space_media_bucket || var.enable_data_class_buckets) ? 1 : 0
  project                     = var.project_id
  name                        = "espacigo-testprod-media-${var.bucket_suffix}"
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  labels = {
    application = "espacigo"
    managed_by  = "terraform"
    purpose     = "profile-space-media"
  }

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket_iam_member" "api_profile_space_media_only" {
  count  = (var.enable_profile_space_media_bucket || var.enable_data_class_buckets) ? 1 : 0
  bucket = google_storage_bucket.profile_space_media_only[0].name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:espacigo-test-api@${var.project_id}.iam.gserviceaccount.com"

  depends_on = [google_project_service.required]
}
