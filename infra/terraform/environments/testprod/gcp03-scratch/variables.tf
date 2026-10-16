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
      var.run_id == "pilot01" &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.migrator_image)) &&
      startswith(var.migrator_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_image_pushed &&
      var.confirm_scratch_secret_version &&
      var.confirm_database_owner_secret_version
    )
    error_message = "The named scratch phases require run_id pilot01, the exact scratch login/database, a sha256-pinned image from this repository, and confirmations that both temporary secret versions exist."
  }
}

variable "enable_role_probe_jobs" {
  type        = bool
  description = "Separate gate for the two fixed V60 role-comment probe Jobs; no migration or arbitrary-SQL phase is included."
  default     = false
  validation {
    condition = !var.enable_role_probe_jobs || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      !var.enable_job &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.role_probe_image)) &&
      startswith(var.role_probe_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_role_probe_image_pushed &&
      var.confirm_role_probe_secret_version &&
      can(regex("^[1-9][0-9]*$", var.role_probe_secret_version)) &&
      can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.role_probe_iam_expires_at))
    )
    error_message = "Role-comment probe Jobs require the exact pilot01 scratch target, uploaded digest image, numeric version, temporary IAM expiry, and explicit operator member."
  }
}

variable "role_probe_image" {
  type        = string
  description = "Digest-pinned GCP-03 image containing only fixed named phases; no mutable tags."
  default     = ""
}

variable "role_probe_secret_version" {
  type        = string
  description = "Concrete numeric Secret Manager version mounted by both probe Jobs; never use latest."
  default     = "1"
  validation {
    condition     = can(regex("^[1-9][0-9]*$", var.role_probe_secret_version))
    error_message = "The fixed role-comment probe requires an immutable numeric secret version."
  }
}

variable "confirm_role_probe_image_pushed" {
  type        = bool
  description = "Confirm only after the clean reviewed probe image digest is pushed and read back from this exact scratch repository."
  default     = false
}

variable "confirm_role_probe_secret_version" {
  type        = bool
  description = "Confirm only after the exact numeric migrator secret version exists and is verified active."
  default     = false
}

variable "role_probe_iam_expires_at" {
  type        = string
  description = "Explicit RFC3339 expiry for the temporary human operator Cloud SQL/secret IAM grants; choose after operation approval."
  default     = ""
  validation {
    condition     = var.role_probe_iam_expires_at == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.role_probe_iam_expires_at))
    error_message = "The enabled role-comment probe requires a valid explicit IAM expiry timestamp."
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

variable "confirm_database_owner_secret_version" {
  type        = bool
  description = "Set only after the exact temporary postgres-owner secret version exists for the isolated database ACL phase."
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
    condition     = !(var.enable_job || var.enable_role_probe_jobs) || can(regex("^(user|group|serviceAccount):[^[:space:]]+$", var.operator_member))
    error_message = "The gated Job requires one explicit user:, group:, or serviceAccount: operator member."
  }
}
