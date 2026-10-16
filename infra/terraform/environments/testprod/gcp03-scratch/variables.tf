variable "project_id" {
  type        = string
  description = "Explicit existing project. This isolated root never creates a project."
  default     = ""
  validation {
    condition     = !var.enable_scratch || var.project_id == "espacigo-testprod"
    error_message = "This scratch root is restricted to the explicitly reviewed existing project espacigo-testprod."
  }
}

variable "region" {
  type        = string
  description = "Region shared with the existing private test VPC."
  default     = "southamerica-west1"
  validation {
    condition     = !var.enable_scratch || var.region == "southamerica-west1"
    error_message = "This reviewed scratch root is restricted to southamerica-west1."
  }
}

variable "network_name" {
  type        = string
  description = "Existing VPC, read only. This root does not own or modify it."
  default     = "espacigo-testprod"
  validation {
    condition     = !var.enable_scratch || var.network_name == "espacigo-testprod"
    error_message = "This root is restricted to reading the pre-existing espacigo-testprod VPC."
  }
}

variable "subnetwork_name" {
  type        = string
  description = "Existing Direct VPC egress subnet, read only."
  default     = "espacigo-testprod-serverless"
  validation {
    condition     = !var.enable_scratch || var.subnetwork_name == "espacigo-testprod-serverless"
    error_message = "This root is restricted to reading the pre-existing serverless subnet."
  }
}

variable "run_id" {
  type        = string
  description = "Unique short rehearsal identifier; never reuse a scratch instance name."
  default     = ""
  validation {
    condition     = !var.enable_scratch || can(regex("^[a-z][a-z0-9-]{5,11}$", var.run_id))
    error_message = "When enabled, run_id must be 6-12 lowercase alphanumeric/hyphen characters and start with a letter."
  }
}

variable "enable_scratch" {
  type        = bool
  description = "Explicit plan/apply gate for this isolated scratch root; remains off by default."
  default     = false
}

variable "enable_job" {
  type        = bool
  description = "Separate gate for the Cloud Run migration Job; requires an uploaded digest image and active temporary secret version."
  default     = false
  validation {
    condition = !var.enable_job || (
      var.enable_scratch &&
      can(regex("@sha256:[0-9a-f]{64}$", var.migrator_image)) &&
      startswith(var.migrator_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_image_pushed &&
      var.confirm_scratch_secret_version
    )
    error_message = "The job requires enabled scratch resources, a sha256-pinned image from this run's exact Artifact Registry repository, and confirmations that the image and temporary secret version exist."
  }
}

variable "migrator_image" {
  type        = string
  description = "Image reference in the scratch Artifact Registry repository, pinned by immutable sha256 digest."
  default     = ""
}

variable "confirm_project_inventory" {
  type        = bool
  description = "Set only after rechecking the exact project, VPC, subnet, and existing Cloud SQL inventory."
  default     = false
  validation {
    condition     = !var.enable_scratch || var.confirm_project_inventory
    error_message = "Scratch resources require a reviewed explicit project and inventory confirmation."
  }
}

variable "confirm_image_pushed" {
  type        = bool
  description = "Set only after the reviewed OCI manifest digest has been pushed to this exact repository and read back."
  default     = false
}

variable "confirm_scratch_secret_version" {
  type        = bool
  description = "Set only after a temporary credential was created outside Terraform and a version exists on the scratch-only secret."
  default     = false
}

variable "allow_scratch_destroy" {
  type        = bool
  description = "Two-step teardown gate: first disable protections while keeping resources enabled, then disable scratch to destroy only this root's resources."
  default     = false
}

variable "confirm_destroy_run_id" {
  type        = string
  description = "Must equal run_id for either step of scratch teardown."
  default     = ""
  validation {
    condition     = !var.allow_scratch_destroy || (var.run_id != "" && var.confirm_destroy_run_id == var.run_id)
    error_message = "Scratch deletion protection may be lowered only when confirm_destroy_run_id exactly matches run_id."
  }
}

variable "instance_tier" {
  type        = string
  description = "Reviewed scratch pairing only: N4 2 vCPU/4 GiB with Hyperdisk Balanced; recheck regional availability before a future apply."
  default     = "db-custom-N4-2-4096"
  validation {
    condition     = !var.enable_scratch || var.instance_tier == "db-custom-N4-2-4096"
    error_message = "This scratch root currently supports only the reviewed N4 2 vCPU/4 GiB tier paired with Hyperdisk Balanced."
  }
}

variable "database_name" {
  type        = string
  description = "Dedicated empty scratch database name; no restore source is configured by this root."
  default     = "espacigo_scratch"
  validation {
    condition     = var.database_name == "espacigo_scratch"
    error_message = "The review-only bootstrap is bound to the exact database name espacigo_scratch."
  }
}

variable "database_user" {
  type        = string
  description = "Temporary PostgreSQL login to be bootstrapped separately; Terraform does not create SQL users."
  default     = "espacigo_scratch_migrator"
  validation {
    condition     = can(regex("^[a-z][a-z0-9_]{0,62}$", var.database_user))
    error_message = "database_user must be a safe lowercase PostgreSQL identifier."
  }
}

variable "operator_member" {
  type        = string
  description = "Exact IAM member allowed to execute the named job and act as its scratch-only service account."
  default     = ""
  validation {
    condition     = !var.enable_job || can(regex("^(user|group|serviceAccount):[^[:space:]]+$", var.operator_member))
    error_message = "The gated migration Job requires one explicit user:, group:, or serviceAccount: operator member."
  }
}
