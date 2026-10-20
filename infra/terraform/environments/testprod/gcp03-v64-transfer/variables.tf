variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "The G03O transfer is restricted to espacigo-testprod."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "The G03O transfer is restricted to southamerica-west1."
  }
}

variable "enable_job" {
  type        = bool
  default     = false
  description = "Default-off gate for the private single-task transfer Job and its scoped IAM."
  validation {
    condition     = !var.enable_job || var.confirm_job_release
    error_message = "Enabling the transfer Job requires its separately reviewed release acknowledgment."
  }
}

variable "confirm_job_release" {
  type    = bool
  default = false
}

variable "transfer_mode" {
  type        = string
  default     = "preflight"
  description = "preflight is read-only; execute is a separate reviewed release that pauses API and mutates the target."
  validation {
    condition     = contains(["preflight", "execute"], var.transfer_mode)
    error_message = "Transfer mode must be preflight or execute."
  }
}

variable "confirm_transfer_execution" {
  type    = bool
  default = false
}

variable "reviewed_release_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.reviewed_release_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.reviewed_release_sha256))
    error_message = "The reviewed release fingerprint must be a lowercase SHA-256."
  }
}

variable "transfer_image" {
  type    = string
  default = ""
  validation {
    condition     = var.transfer_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.transfer_image)) && startswith(var.transfer_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/"))
    error_message = "The transfer image must be a digest-pinned image in the testprod Artifact Registry."
  }
}

variable "proxy_image" {
  type    = string
  default = ""
  validation {
    condition     = var.proxy_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.proxy_image)) && startswith(var.proxy_image, "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:"))
    error_message = "Both Cloud SQL Auth Proxy sidecars must use a digest-pinned official image."
  }
}

variable "source_admin_secret_version" {
  type    = string
  default = ""
}

variable "source_transport_secret_version" {
  type    = string
  default = ""
}

variable "target_migrator_secret_version" {
  type    = string
  default = ""
}

variable "receipt_sha256" {
  type    = string
  default = ""
}

variable "receipt_generation" {
  type    = string
  default = ""
}

variable "receipt_bytes" {
  type    = number
  default = 0
}

variable "manifest_sha256" {
  type    = string
  default = ""
}

variable "bootstrap_report_sha256" {
  type    = string
  default = ""
}

variable "observer_report_sha256" {
  type    = string
  default = ""
}

variable "normalization_sql_sha256" {
  type    = string
  default = ""
}

variable "confirm_source_secrets" {
  type        = bool
  default     = false
  description = "Confirms the exact existing numeric G03O admin/transport versions were independently verified without reading values."
}

variable "confirm_target_migrator_secret" {
  type        = bool
  default     = false
  description = "Confirms the exact existing target migrator secret version and login."
}

variable "confirm_target_files_bucket" {
  type        = bool
  default     = false
  description = "Confirms the current API uses the exact existing private files bucket."
}
