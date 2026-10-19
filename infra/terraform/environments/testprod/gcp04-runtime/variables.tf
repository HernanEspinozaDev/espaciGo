variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "This runtime root is restricted to the existing espacigo-testprod project."
  }
}
variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "This runtime root is restricted to southamerica-west1."
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
variable "sql_instance_name" {
  type    = string
  default = "espacigo-testprod-pg"
}
variable "artifact_registry_repository_id" {
  type    = string
  default = "espacigo-testprod"
}
variable "database_name" {
  type    = string
  default = "espacigo"
}
variable "enable_worker" {
  type    = bool
  default = false
  validation {
    condition     = !var.enable_worker || (var.confirm_runtime_database_bootstrap && var.confirm_connection_budget && var.confirm_cost_review)
    error_message = "Worker runtime requires reviewed SQL bootstrap, connection budget and cost confirmations."
  }
}
variable "enable_migrator" {
  type    = bool
  default = false
  validation {
    condition     = !var.enable_migrator || (var.confirm_runtime_database_bootstrap && var.confirm_connection_budget && var.confirm_cost_review)
    error_message = "Migrator runtime requires reviewed SQL bootstrap, connection budget and cost confirmations."
  }
}
variable "enable_bootstrap" {
  type    = bool
  default = false
  validation {
    condition     = !var.enable_bootstrap || (var.confirm_runtime_database_bootstrap && var.confirm_connection_budget && var.confirm_cost_review)
    error_message = "W7 bootstrap requires reviewed SQL bootstrap, connection budget and cost confirmations."
  }
}
variable "manage_migrator_secret_iam" {
  type    = bool
  default = true
}
variable "confirm_runtime_database_bootstrap" {
  type    = bool
  default = false
}
variable "confirm_connection_budget" {
  type    = bool
  default = false
}
variable "confirm_cost_review" {
  type    = bool
  default = false
}
variable "confirm_baseline_restore" {
  type    = bool
  default = false
}
variable "baseline_manifest_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.baseline_manifest_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.baseline_manifest_sha256))
    error_message = "Pin the exact private W6 baseline manifest SHA-256."
  }
}
variable "baseline_dump_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.baseline_dump_sha256 == "" || var.baseline_dump_sha256 == "1d31721c1a3fee15903f3a5cfbb348e5803b567727187c0a8871313b94f7faee"
    error_message = "Only the retained W6 baseline archive may be selected."
  }
}
variable "worker_pool_name" {
  type    = string
  default = "espacigo-test-privacy-worker"
}
variable "migrator_job_name" {
  type    = string
  default = "espacigo-test-migrator"
}
variable "worker_service_account_id" {
  type    = string
  default = "espacigo-test-privacy-worker"
}
variable "migrator_service_account_email" {
  type    = string
  default = ""
}
variable "worker_database_user" {
  type    = string
  default = ""
}
variable "migrator_database_user" {
  type    = string
  default = ""
}
variable "worker_database_password_secret_id" {
  type    = string
  default = ""
}
variable "worker_database_password_version" {
  type    = string
  default = ""
}
variable "migrator_database_password_secret_id" {
  type    = string
  default = ""
}
variable "migrator_database_password_version" {
  type    = string
  default = ""
}
variable "api_database_user" {
  type    = string
  default = ""
}
variable "bootstrap_database_user" {
  type    = string
  default = ""
}
variable "api_database_password_secret_id" {
  type    = string
  default = ""
}
variable "api_database_password_version" {
  type    = string
  default = ""
}
variable "bootstrap_database_password_secret_id" {
  type    = string
  default = ""
}
variable "bootstrap_database_password_version" {
  type    = string
  default = ""
}
variable "bootstrap_job_name" {
  type    = string
  default = "espacigo-test-runtime-bootstrap"
}
variable "bootstrap_mode" {
  type    = string
  default = "bootstrap"
}
variable "private_bucket_name" {
  type    = string
  default = ""
}
variable "private_object_prefix" {
  type    = string
  default = "private/evidence"
}
variable "worker_image" {
  type    = string
  default = ""
}
variable "migrator_image" {
  type    = string
  default = ""
}
variable "migrator_operation" {
  type    = string
  default = "migrate"
  validation {
    condition     = contains(["migrate", "baseline-restore"], var.migrator_operation)
    error_message = "The existing migrator Job supports only normal migrations or the single transactional W6 baseline restore."
  }
}
variable "baseline_restore_image" {
  type    = string
  default = ""
  validation {
    condition     = var.baseline_restore_image == "" || can(regex("@sha256:[0-9a-f]{64}$", var.baseline_restore_image))
    error_message = "The W6 restore executor image must be pinned by immutable digest."
  }
}
variable "bootstrap_image" {
  type    = string
  default = ""
}
