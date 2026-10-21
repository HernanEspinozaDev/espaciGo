terraform {
  required_version = ">= 1.16.0, < 1.17.0"

  backend "gcs" {
    bucket = "espacigo-testprod-tfstate-447373903153-gcp"
    prefix = "espacigo/testprod/gcp08-bootstrap"
  }

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.7.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
