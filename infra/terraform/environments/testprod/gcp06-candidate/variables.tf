variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "GCP-06 candidate resources are restricted to the testprod project."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "GCP-06 candidate resources are restricted to Santiago."
  }
}

variable "network_name" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.network_name == "espacigo-testprod"
    error_message = "GCP-06 candidates must use the existing testprod private network."
  }
}

variable "run_id" {
  type    = string
  default = "pending"
  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{5,20}$", var.run_id))
    error_message = "Use a unique lowercase run id between 6 and 21 characters."
  }
}

variable "enable_candidate" {
  type        = bool
  default     = false
  description = "Default-off gate for a new isolated V64 Cloud SQL candidate and private import bucket."
  validation {
    condition     = !var.enable_candidate || var.confirm_candidate_release
    error_message = "Creating the candidate requires its separate reviewed release confirmation."
  }
}

variable "confirm_candidate_release" {
  type    = bool
  default = false
}

variable "instance_tier" {
  type        = string
  default     = "db-custom-1-3840"
  description = "One vCPU / 3.75 GiB for an isolated test candidate; revisit after a measured rehearsal."
}

variable "disk_gib" {
  type    = number
  default = 20
  validation {
    condition     = var.disk_gib == floor(var.disk_gib) && var.disk_gib >= 20 && var.disk_gib <= 100
    error_message = "Candidate disk must be an integer from 20 through 100 GiB."
  }
}

variable "allow_candidate_destroy" {
  type        = bool
  default     = false
  description = "Deletion gate. Keep false until candidate review and explicit cleanup approval."
}

variable "confirm_candidate_destroy_run_id" {
  type    = string
  default = ""
}

variable "enable_import_binding" {
  type        = bool
  default     = false
  description = "Temporary exact-object read binding for the Cloud SQL import service identity."
  validation {
    condition     = !var.enable_import_binding || (var.enable_candidate && var.confirm_candidate_release && var.confirm_import_binding && can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.import_binding_expires_at)) && try(timecmp(timestamp(), var.import_binding_expires_at) < 0 && timecmp(var.import_binding_expires_at, timeadd(timestamp(), "48h")) <= 0, false))
    error_message = "Import access requires confirmation and a future UTC expiration no more than 48 hours away."
  }
}

variable "confirm_import_binding" {
  type    = bool
  default = false
}

variable "import_binding_expires_at" {
  type    = string
  default = ""
}

variable "enable_upload_binding" {
  type        = bool
  default     = false
  description = "Temporary exact-object create binding for the specifically named uploader principal."
  validation {
    condition     = !var.enable_upload_binding || (var.enable_candidate && var.confirm_candidate_release && var.confirm_upload_binding && can(regex("^(user|serviceAccount):[^ ]+$", var.uploader_member)) && can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.upload_binding_expires_at)) && try(timecmp(timestamp(), var.upload_binding_expires_at) < 0 && timecmp(var.upload_binding_expires_at, timeadd(timestamp(), "48h")) <= 0, false))
    error_message = "Upload access requires confirmation, a specific member and future UTC expiration no more than 48 hours away."
  }
}

variable "confirm_upload_binding" {
  type    = bool
  default = false
}

variable "uploader_member" {
  type        = string
  default     = ""
  description = "Exact IAM member for the short-lived object-create permission."
}

variable "upload_binding_expires_at" {
  type    = string
  default = ""
}
