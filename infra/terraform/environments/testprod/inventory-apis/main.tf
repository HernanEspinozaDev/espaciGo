terraform {
  required_version = ">= 1.16.0, < 1.17.0"

  backend "local" {}

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.7.0"
    }
  }
}

provider "google" {
  project = var.project_id
}

resource "google_project_service" "inventory" {
  for_each = toset(var.services_to_enable)

  project                    = var.project_id
  service                    = each.value
  disable_dependent_services = false
  disable_on_destroy         = false
}
