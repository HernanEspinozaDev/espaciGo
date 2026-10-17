variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "GCP-03 origin foundation is restricted to espacigo-testprod."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "GCP-03 origin foundation is restricted to southamerica-west1."
  }
}

variable "run_id" {
  type    = string
  default = ""
  validation {
    condition     = !var.enable_foundation || can(regex("^[a-z][a-z0-9-]{5,11}$", var.run_id))
    error_message = "An enabled foundation requires a new 6-12 character lowercase run_id."
  }

}

variable "enable_foundation" {
  type        = bool
  default     = false
  description = "Default-off gate for the run-scoped Cloud SQL instance, empty database, artifact bucket and bootstrap secret metadata."
  validation {
    condition     = !var.enable_foundation || var.confirm_origin_rehearsal
    error_message = "Foundation requires explicit confirmation that this is an isolated origin rehearsal."
  }
}

variable "confirm_origin_rehearsal" {
  type        = bool
  default     = false
  description = "Explicit acknowledgment of the new, private, run-scoped origin rehearsal target."
}

variable "enable_origin_bootstrap_job" {
  type        = bool
  default     = false
  description = "Separate default-off gate for the one-shot origin bootstrap Job. It creates no secret version and does not execute the Job."
  validation {
    condition     = !var.enable_origin_bootstrap_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && can(regex("@sha256:[0-9a-f]{64}$", var.origin_executor_image)) && can(regex("^[1-9][0-9]*$", var.admin_password_secret_version)) && var.confirm_admin_password_secret_version)
    error_message = "Origin bootstrap requires the exact g03o-1017 Foundation, digest-pinned executor, and reviewed numeric admin-password version."
  }
}

variable "enable_origin_observer_job" {
  type        = bool
  default     = false
  description = "Separate default-off gate for the IAM-only pre-restore observer; requires bootstrap to be completed and reviewed first."
  validation {
    condition     = !var.enable_origin_observer_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && var.confirm_origin_bootstrap_completed && can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256)) && can(regex("@sha256:[0-9a-f]{64}$", var.origin_executor_image)))
    error_message = "Origin observer requires the exact g03o-1017 Foundation, a reviewed digest, and hash-bound evidence that bootstrap completed."
  }
}

variable "origin_executor_image" {
  type        = string
  default     = ""
  description = "Immutable SHA-256 image digest for the separate gcp03origin executor, stored in the existing scratch Artifact Registry repository."
  validation {
    condition     = var.origin_executor_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_executor_image)) && startswith(var.origin_executor_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:"))
    error_message = "Origin executor image must be a SHA-256 digest in the existing reviewed GCP-03 Artifact Registry repository."
  }
}

variable "admin_password_secret_version" {
  type        = string
  default     = ""
  description = "Exact numeric Secret Manager version for a future bootstrap plan; never use latest."
  validation {
    condition     = var.admin_password_secret_version == "" || can(regex("^[1-9][0-9]*$", var.admin_password_secret_version))
    error_message = "The bootstrap password secret version must be a positive numeric version."
  }
}

variable "confirm_admin_password_secret_version" {
  type        = bool
  default     = false
  description = "Acknowledges that the exact active admin-password version was provisioned out of band and reviewed."
}

variable "confirm_origin_bootstrap_completed" {
  type        = bool
  default     = false
  description = "Acknowledges successful bootstrap evidence before creating the observer IAM database user and Job."
}

variable "bootstrap_evidence_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of the reviewed sanitized bootstrap report required before creating the observer IAM user/Job."
  validation {
    condition     = var.bootstrap_evidence_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256))
    error_message = "Bootstrap evidence must be identified by a lowercase SHA-256 digest."
  }
}

locals {
  foundation_enabled    = var.enable_foundation && var.confirm_origin_rehearsal
  prefix                = "espacigo-gcp03-origin-${var.run_id}"
  admin_account_id      = "g03o-admin-${var.run_id}"
  instance_name         = "${local.prefix}-pg"
  admin_enabled         = local.foundation_enabled && var.enable_origin_bootstrap_job
  observer_enabled      = local.foundation_enabled && var.enable_origin_observer_job
  observer_account_id   = "g03o-observer-${var.run_id}"
  observer_database_url = "postgres://${local.observer_account_id}%40${var.project_id}.iam@localhost:5432/espacigo_scratch?sslmode=disable"
}
