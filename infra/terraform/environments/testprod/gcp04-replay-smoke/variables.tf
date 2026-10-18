variable "project_id" {
  type        = string
  description = "Exact existing test project. This root never creates or changes billing/projects."
  default     = ""
  validation {
    condition     = !var.enable_base || var.project_id == "espacigo-testprod"
    error_message = "GCP-04 smoke resources are restricted to the explicitly reviewed espacigo-testprod project."
  }
}

variable "region" {
  type        = string
  description = "Shared region for the existing private test VPC, Cloud SQL, Cloud Run Job, bucket, and Artifact Registry."
  default     = "southamerica-west1"
  validation {
    condition     = !var.enable_base || var.region == "southamerica-west1"
    error_message = "This rehearsal is restricted to southamerica-west1."
  }
}

variable "run_id" {
  type        = string
  description = "Unique short run ID used in every disposable resource name and object prefix."
  default     = ""
  validation {
    condition     = !var.enable_base || can(regex("^[a-z][a-z0-9-]{5,11}$", var.run_id))
    error_message = "When enabled, run_id must contain 6-12 lowercase letters, digits or hyphens and start with a letter."
  }
}

variable "bucket_suffix" {
  type        = string
  description = "Globally unique checked suffix for the test bucket. Keep empty until availability is checked."
  default     = ""
  validation {
    condition     = !var.enable_base || can(regex("^[a-z0-9][a-z0-9-]{2,24}$", var.bucket_suffix))
    error_message = "A reviewed globally unique bucket suffix is required before the base plan."
  }
}

variable "enable_base" {
  type        = bool
  description = "Default-off gate for one disposable PostgreSQL 18 instance, private bucket, secret metadata, repository and scoped runner identity."
  default     = false
}

variable "confirm_isolated_inventory" {
  type        = bool
  description = "Human confirmation that the exact run-scoped resource names are absent and the existing VPC/subnet are the reviewed shared network."
  default     = false
  validation {
    condition     = !var.enable_base || var.confirm_isolated_inventory
    error_message = "Enable the base only after a fresh read-only inventory confirms every resource name and the existing network."
  }
}

variable "enable_job" {
  type        = bool
  description = "Separate default-off gate for the digest-pinned, one-task Cloud Run integration Job."
  default     = false
  validation {
    condition     = !var.enable_job || var.enable_base
    error_message = "The Job requires the already-applied isolated base resources."
  }
}

variable "runner_image" {
  type        = string
  description = "Existing Artifact Registry image by remote sha256 digest; build/publish is a separate reviewed action."
  default     = ""
  validation {
    condition     = !var.enable_job || (can(regex("@sha256:[0-9a-f]{64}$", var.runner_image)) && startswith(var.runner_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp04-replay-${var.run_id}/smoke@sha256:"))
    error_message = "The enabled Job requires a remote sha256 digest from this run's dedicated repository."
  }
}

variable "confirm_image_digest" {
  type        = bool
  description = "Set only after independently verifying the uploaded image's remote digest against the reviewed source archive."
  default     = false
  validation {
    condition     = !var.enable_job || var.confirm_image_digest
    error_message = "The Cloud Run Job requires a verified immutable remote image digest."
  }
}

variable "database_owner_secret_version" {
  type        = string
  description = "Active numeric Secret Manager version created out-of-band; Terraform manages metadata only."
  default     = ""
  validation {
    condition     = !var.enable_job || can(regex("^[1-9][0-9]*$", var.database_owner_secret_version))
    error_message = "The Job requires an active numeric database-owner password version, never a secret value in Terraform."
  }
}

variable "operator_member" {
  type        = string
  description = "Exact human principal permitted to invoke the disposable test Job."
  default     = "user:pasteleriahijitos@gmail.com"
}

variable "allow_job_destroy" {
  type        = bool
  description = "Separate teardown gate for only the Cloud Run Job and its invoker binding."
  default     = false
}

variable "confirm_job_destroy_run_id" {
  type        = string
  description = "Must equal run_id before lowering deletion protection on the Job."
  default     = ""
  validation {
    condition     = !var.allow_job_destroy || (var.run_id != "" && var.confirm_job_destroy_run_id == var.run_id)
    error_message = "Job teardown requires its exact run_id confirmation."
  }
}

variable "allow_base_destroy" {
  type        = bool
  description = "Separate teardown gate for only this run's SQL, bucket, repository, secret metadata, service account and IAM."
  default     = false
}

variable "confirm_base_destroy_run_id" {
  type        = string
  description = "Must equal run_id before lowering deletion protection on base resources."
  default     = ""
  validation {
    condition     = !var.allow_base_destroy || (var.run_id != "" && var.confirm_base_destroy_run_id == var.run_id)
    error_message = "Base teardown requires its exact run_id confirmation."
  }
}

variable "instance_tier" {
  type        = string
  description = "Bounded single-zone Cloud SQL size selected for the short smoke rehearsal."
  default     = "db-custom-N4-2-4096"
  validation {
    condition     = var.instance_tier == "db-custom-N4-2-4096"
    error_message = "The current rehearsal is limited to the reviewed N4 2-vCPU/4-GiB instance size."
  }
}

variable "disk_gib" {
  type        = number
  description = "Minimum run-scoped database disk size; automatic growth is disabled."
  default     = 20
  validation {
    condition     = var.disk_gib == 20
    error_message = "The rehearsal disk is fixed at 20 GiB."
  }
}
