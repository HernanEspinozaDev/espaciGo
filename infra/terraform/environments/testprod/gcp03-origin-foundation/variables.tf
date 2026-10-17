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
  validation {
    condition = !var.allow_origin_destroy || (
      can(regex("^[a-z][a-z0-9-]{5,11}$", var.run_id)) &&
      var.confirm_destroy_run_id == var.run_id
    )
    error_message = "Lowering deletion protection requires confirm_destroy_run_id to equal the exact run_id."
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

variable "allow_origin_destroy" {
  type        = bool
  default     = false
  description = "Run-bound gate to lower protection in a separately reviewed teardown plan; never enable during creation."
}

variable "confirm_destroy_run_id" {
  type        = string
  default     = ""
  description = "Must equal run_id before a separately reviewed plan lowers deletion protection."
}

locals {
  foundation_enabled = var.enable_foundation && var.confirm_origin_rehearsal
  prefix             = "espacigo-gcp03-origin-${var.run_id}"
  admin_account_id   = "g03o-admin-${var.run_id}"
  admin_email        = "${local.admin_account_id}@${var.project_id}.iam.gserviceaccount.com"
  instance_name      = "${local.prefix}-pg"
}
