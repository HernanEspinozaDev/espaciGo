terraform {
  required_version = ">= 1.16.0, < 1.17.0"

  backend "gcs" {}

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.7.0"
    }
  }
}

provider "google" {
  project = var.project_id != "" ? var.project_id : null
  region  = var.region
}

resource "google_storage_bucket" "terraform_state" {
  for_each                    = var.create_state_bucket ? var.state_bucket_names : {}
  project                     = var.project_id
  name                        = each.value
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }

}

resource "google_storage_bucket_iam_member" "state_operator" {
  for_each = var.create_state_bucket ? var.state_operator_members : {}
  bucket   = google_storage_bucket.terraform_state[each.key].name
  role     = "roles/storage.objectAdmin"
  member   = each.value
}
