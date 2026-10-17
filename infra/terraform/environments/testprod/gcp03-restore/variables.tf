variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "The synthetic restore root is restricted to espacigo-testprod."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "The synthetic restore root is restricted to southamerica-west1."
  }
}

variable "run_id" {
  type    = string
  default = ""
  validation {
    condition     = !(var.enable_foundation || var.enable_bootstrap_job || var.enable_restore_jobs) || can(regex("^[a-z][a-z0-9-]{5,11}$", var.run_id))
    error_message = "An enabled run needs a unique 6-12 character lowercase run_id."
  }
}

variable "enable_foundation" {
  type        = bool
  description = "Default-off gate for the separate instance, database, artifact bucket, admin identity, and secret metadata."
  default     = false
}

variable "enable_bootstrap_job" {
  type        = bool
  description = "Separate gate for the one-shot bootstrap Job, after the exact admin secret version and image digests are reviewed."
  default     = false
  validation {
    condition = !var.enable_bootstrap_job || (
      var.enable_foundation && var.confirm_isolated_restore &&
      can(regex("@sha256:[0-9a-f]{64}$", var.executor_image)) &&
      can(regex("^gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:[0-9a-f]{64}$", var.proxy_image)) &&
      can(regex("^[1-9][0-9]*$", var.admin_password_secret_version))
    )
    error_message = "Bootstrap requires the exact foundation gate, explicit isolation confirmation, pinned images, and an existing numeric admin secret version."
  }
}

variable "enable_restore_jobs" {
  type        = bool
  description = "Separate gate for restore, verify, and retire Jobs plus IAM SQL users. Requires separately confirmed successful bootstrap."
  default     = false
  validation {
    condition = !var.enable_restore_jobs || (
      var.enable_foundation && var.confirm_isolated_restore && var.confirm_bootstrap_complete &&
      can(regex("@sha256:[0-9a-f]{64}$", var.executor_image)) &&
      can(regex("^gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:[0-9a-f]{64}$", var.proxy_image)) &&
      can(regex("^[0-9a-f]{64}$", var.manifest_sha256)) &&
      can(regex("^[0-9a-f]{64}$", var.migration_inventory_sha256)) &&
      can(regex("^[1-9][0-9]*$", var.admin_password_secret_version))
    )
    error_message = "Restore jobs require the isolated foundation, explicit prior-bootstrap confirmation, exact artifact/migration hashes, pinned images, and the active numeric admin secret version."
  }
}

variable "enable_restore_iam_users" {
  type        = bool
  default     = true
  description = "Keep the restore and verify Cloud SQL IAM login resources while restore jobs are enabled; set false only for a run-bound IAM-user closeout plan."
  validation {
    condition = var.enable_restore_iam_users || (
      var.allow_restore_iam_user_destroy && var.confirm_destroy_run_id == var.run_id
    )
    error_message = "Disabling the restore/verify IAM SQL users requires the scoped destroy gate and exact run confirmation."
  }
}

variable "enable_diagnostic_job" {
  type        = bool
  default     = false
  description = "Separate default-off gate for a read-only artifact and database-catalog diagnostic Job."
  validation {
    condition     = !var.enable_diagnostic_job || (var.enable_foundation && var.enable_restore_jobs && var.confirm_isolated_restore && can(regex("^[0-9a-f]{64}$", var.manifest_sha256)) && can(regex("@sha256:[0-9a-f]{64}$", var.diagnostic_image)))
    error_message = "The diagnostic Job requires the isolated run, verifier identity, reviewed artifact manifest hash, and an immutable diagnostic image digest."
  }
}

variable "diagnostic_image" {
  type        = string
  default     = ""
  description = "Reviewed immutable image digest for the one-shot read-only GCP-03 diagnostic Job."
  validation {
    condition     = var.diagnostic_image == "" || can(regex("@sha256:[0-9a-f]{64}$", var.diagnostic_image))
    error_message = "A diagnostic image, when supplied, must be pinned by sha256 digest."
  }
}

variable "enable_retirement_diagnostic_job" {
  type        = bool
  default     = false
  description = "Default-off gate for a catalog-only retirement diagnosis Job using the existing run-scoped admin identity. Terraform creates it but never executes it."
  validation {
    condition = !var.enable_retirement_diagnostic_job || (
      var.enable_foundation && var.enable_restore_jobs && var.confirm_isolated_restore &&
      can(regex("@sha256:[0-9a-f]{64}$", var.retirement_diagnostic_image))
    )
    error_message = "Retirement diagnosis requires the isolated restore run, its existing admin identity, and an immutable diagnostic image digest."
  }
}

variable "retirement_diagnostic_image" {
  type        = string
  default     = ""
  description = "Reviewed immutable executor digest for a read-only retirement-state diagnosis Job."
  validation {
    condition     = var.retirement_diagnostic_image == "" || can(regex("@sha256:[0-9a-f]{64}$", var.retirement_diagnostic_image))
    error_message = "A retirement diagnostic image, when supplied, must be pinned by sha256 digest."
  }
}

variable "restore_executor_image" {
  type        = string
  default     = ""
  description = "Optional reviewed immutable image override for the restore Job only; set only after a read-only baseline and a separate recovery review."
  validation {
    condition = var.restore_executor_image == "" || (
      can(regex("@sha256:[0-9a-f]{64}$", var.restore_executor_image)) &&
      var.enable_restore_jobs && var.confirm_bootstrap_complete && var.confirm_isolated_restore &&
      var.confirm_read_only_empty_baseline && can(regex("^[0-9a-f]{64}$", var.diagnostic_evidence_sha256))
    )
    error_message = "A restore-only image override requires the isolated restore gates, completed bootstrap, a confirmed empty read-only baseline, and its evidence SHA-256."
  }
}

variable "confirm_read_only_empty_baseline" {
  type        = bool
  default     = false
  description = "Set only after reviewing diagnostic evidence that the target satisfies the fixed empty-target precondition."
}

variable "diagnostic_evidence_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of the sanitized read-only diagnostic result required for any restore image override."
  validation {
    condition     = var.diagnostic_evidence_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.diagnostic_evidence_sha256))
    error_message = "Diagnostic evidence binding must be empty or a lowercase SHA-256 digest."
  }
}

variable "confirm_bootstrap_complete" {
  type        = bool
  description = "Operator acknowledgment only after bootstrap Job evidence verifies roles, extensions, and target ACLs."
  default     = false
}

variable "confirm_isolated_restore" {
  type        = bool
  description = "Explicit acknowledgment that this root creates a separate private Cloud SQL instance and run-scoped bucket."
  default     = false
}

variable "allow_restore_destroy" {
  type        = bool
  default     = false
  description = "Run-bound gate that first lowers Cloud SQL and Cloud Run deletion protections."
  validation {
    condition     = !(var.allow_restore_destroy || var.allow_restore_iam_user_destroy) || var.confirm_destroy_run_id == var.run_id
    error_message = "Destroy protection can be lowered only when confirm_destroy_run_id exactly matches run_id."
  }
}

variable "allow_restore_iam_user_destroy" {
  type        = bool
  default     = false
  description = "Run-bound gate for deleting only the two Terraform-managed Cloud SQL IAM login resources during isolated-run closeout; does not lower instance or Job deletion protections."
}

variable "confirm_destroy_run_id" {
  type        = string
  default     = ""
  description = "Must exactly match run_id before a separately reviewed teardown sequence."
}

variable "executor_image" {
  type        = string
  default     = ""
  description = "Reviewed immutable multi-phase executor image digest. No image is built or pushed by Terraform."
}

variable "manifest_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of the exact synthetic run manifest; supplied only after fixture/dump/TOC review."
}

variable "proxy_image" {
  type        = string
  default     = ""
  description = "Reviewed immutable Cloud SQL Auth Proxy OCI digest; mutable tags are rejected."
}

variable "admin_password_secret_version" {
  type        = string
  default     = ""
  description = "Exact existing numeric Secret Manager version; secret payloads and versions are never Terraform-managed."
}

variable "bootstrap_operator_member" {
  type        = string
  default     = "user:pasteleriahijitos@gmail.com"
  description = "The single authorized g03r-1016 operator receiving invocation on exact gated Jobs and actAs on their service accounts."
  validation {
    condition     = !(var.enable_bootstrap_job || var.enable_restore_jobs) || var.bootstrap_operator_member == "user:pasteleriahijitos@gmail.com"
    error_message = "GCP-03 gated Jobs are restricted to the authorized g03r-1016 operator identity."
  }
}

variable "migration_inventory_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of immutable V1-V61 inventory, reviewed with the exact dump manifest."
}

locals {
  prefix             = "espacigo-gcp03-restore-${var.run_id}"
  foundation_enabled = var.enable_foundation && var.confirm_isolated_restore
  executor_digest_valid = (
    can(regex("@sha256:[0-9a-f]{64}$", var.executor_image)) &&
    can(regex("^gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:[0-9a-f]{64}$", var.proxy_image)) &&
    can(regex("^[1-9][0-9]*$", var.admin_password_secret_version))
  )
  bootstrap_job_enabled = local.foundation_enabled && var.enable_bootstrap_job && local.executor_digest_valid
  restore_enabled = (
    local.foundation_enabled && var.enable_restore_jobs && var.confirm_bootstrap_complete &&
    local.executor_digest_valid && can(regex("^[0-9a-f]{64}$", var.manifest_sha256)) &&
    can(regex("^[0-9a-f]{64}$", var.migration_inventory_sha256))
  )
  diagnostic_enabled            = local.restore_enabled && var.enable_diagnostic_job
  retirement_diagnostic_enabled = local.restore_enabled && var.enable_retirement_diagnostic_job
  restore_phase_image           = var.restore_executor_image != "" ? var.restore_executor_image : var.executor_image
  owner_role                    = "g03_restore_owner_${replace(var.run_id, "-", "_")}"
  verifier_role                 = "g03_restore_verify_${replace(var.run_id, "-", "_")}"
  restore_service_account_email = "g03-restore-${var.run_id}@${var.project_id}.iam.gserviceaccount.com"
  verify_service_account_email  = "g03-verify-${var.run_id}@${var.project_id}.iam.gserviceaccount.com"
  admin_service_account_email   = "g03-restore-admin-${var.run_id}@${var.project_id}.iam.gserviceaccount.com"
  admin_jobs = merge(
    local.bootstrap_job_enabled ? { bootstrap = "bootstrap" } : {},
    local.restore_enabled ? { retire = "retire" } : {}
  )
}

data "google_compute_network" "existing" {
  count   = local.foundation_enabled ? 1 : 0
  name    = "espacigo-testprod"
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  count   = local.foundation_enabled ? 1 : 0
  name    = "espacigo-testprod-serverless"
  region  = var.region
  project = var.project_id
}
