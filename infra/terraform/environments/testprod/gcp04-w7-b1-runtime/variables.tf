variable "enable_runtime" {
  type    = bool
  default = false
}

variable "confirm_release" {
  type    = bool
  default = false
}

variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "W7 B1 target project is fixed."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "W7 B1 must run in the Cloud SQL region."
  }
}

variable "network_name" {
  type    = string
  default = "espacigo-testprod"
}

variable "subnetwork_name" {
  type    = string
  default = "espacigo-testprod-serverless"
}

variable "run_id" {
  type    = string
  default = ""
  validation {
    condition     = var.run_id == "" || can(regex("^[a-z][a-z0-9-]{2,39}$", var.run_id))
    error_message = "Use a unique reviewed lowercase run ID."
  }
}

variable "service_account_id" {
  type    = string
  default = "espacigo-gcp04-w7-b1-probe"
}

variable "job_name" {
  type    = string
  default = "espacigo-gcp04-w7-b1-probe"
}

variable "image_digest" {
  type    = string
  default = ""
  validation {
    condition     = var.image_digest == "" || can(regex("^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:[0-9a-f]{64}$", var.image_digest))
    error_message = "Use the published W7 B1 image by immutable digest."
  }
}

variable "database_password_secret_id" {
  type    = string
  default = "espacigo-gcp04-w7-b1-bootstrap-credential"
  validation {
    condition     = var.database_password_secret_id == "espacigo-gcp04-w7-b1-bootstrap-credential"
    error_message = "W7 B1 credential secret is fixed."
  }
}

variable "database_password_secret_version" {
  type      = string
  default   = ""
  sensitive = true
  validation {
    condition     = var.database_password_secret_version == "" || can(regex("^[1-9][0-9]*$", var.database_password_secret_version))
    error_message = "Specify a numeric secret version only in an approved release."
  }
}

variable "release_json" {
  type      = string
  default   = ""
  sensitive = true
}

variable "release_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.release_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.release_sha256))
    error_message = "The approved release SHA-256 must be lowercase hex."
  }
}

variable "marker_bucket_name" {
  type    = string
  default = ""
  validation {
    condition     = var.marker_bucket_name == "" || can(regex("^[a-z0-9][a-z0-9._-]{1,61}[a-z0-9]$", var.marker_bucket_name))
    error_message = "Set the exact pre-existing private bucket only for an approved run."
  }
}

variable "binary_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.binary_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.binary_sha256))
    error_message = "Use the exact runtime binary SHA-256 for this release."
  }
}
