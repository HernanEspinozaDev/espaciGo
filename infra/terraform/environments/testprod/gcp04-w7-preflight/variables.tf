variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "W7 is fixed to the existing EspaciGo test project."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "W7 must run beside the existing private SQL instance in Santiago."
  }
}

variable "sql_instance_name" {
  type    = string
  default = "espacigo-testprod-pg"
  validation {
    condition     = var.sql_instance_name == "espacigo-testprod-pg"
    error_message = "W7 is fixed to the observed existing Cloud SQL instance."
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

variable "database_name" {
  type    = string
  default = "espacigo"
  validation {
    condition     = var.database_name == "espacigo"
    error_message = "W7 is fixed to the observed espacigo database."
  }
}

variable "enable_preflight" {
  type    = bool
  default = false
}

variable "confirm_preflight_release" {
  type    = bool
  default = false
}

variable "preflight_job_name" {
  type    = string
  default = "espacigo-gcp04-w7-preflight"
}

variable "preflight_service_account_id" {
  type    = string
  default = "espacigo-gcp04-w7-preflight"
}

variable "preflight_database_user" {
  type    = string
  default = "espacigo_gcp04_w7_observer"
  validation {
    condition     = var.preflight_database_user == "espacigo_gcp04_w7_observer"
    error_message = "Use the dedicated, proposed W7 observer login only."
  }
}

variable "preflight_database_password_secret_id" {
  type    = string
  default = "espacigo-gcp04-w7-observer-password"
}

variable "preflight_database_password_version" {
  type    = string
  default = ""
  validation {
    condition     = var.preflight_database_password_version == "" || can(regex("^[1-9][0-9]*$", var.preflight_database_password_version))
    error_message = "Select the exact numeric secret version only at an authorized release."
  }
}

variable "preflight_expected_private_ip" {
  type    = string
  default = "10.189.0.3"
  validation {
    condition     = var.preflight_expected_private_ip == "10.189.0.3"
    error_message = "Refresh the metadata and review the runner target pin before changing the private IP."
  }
}

variable "required_connections" {
  type    = number
  default = 0
  validation {
    condition     = var.required_connections == floor(var.required_connections) && var.required_connections >= 0 && var.required_connections <= 999999
    error_message = "The reviewed connection budget must be an integer from 1 to 999999 when enabled."
  }
}

variable "timeout_seconds" {
  type    = number
  default = 120
  validation {
    condition     = var.timeout_seconds == floor(var.timeout_seconds) && var.timeout_seconds >= 60 && var.timeout_seconds <= 900
    error_message = "W7 timeout must be an integer from 60 through 900 seconds."
  }
}

variable "preflight_image" {
  type    = string
  default = ""
  validation {
    condition     = var.preflight_image == "" || can(regex("^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp04-preflight@sha256:[0-9a-f]{64}$", var.preflight_image))
    error_message = "The enabled job requires the W7 Artifact Registry image pinned to its immutable digest."
  }
}

variable "artifact_registry_repository_id" {
  type    = string
  default = "espacigo-testprod"
}
