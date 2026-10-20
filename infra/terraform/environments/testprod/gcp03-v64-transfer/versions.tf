terraform {
  required_version = ">= 1.16.0, < 1.17.0"
  backend "gcs" {
    bucket = "espacigo-testprod-tfstate-447373903153-gcp"
    prefix = "espacigo/testprod/g03o-1017-v64-transfer/20261020"
  }
  required_providers {
    google = { source = "hashicorp/google", version = "~> 8.7.0" }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
