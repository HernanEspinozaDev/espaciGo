variable "project_id" {
  type        = string
  description = "Existing project ID; this root never creates a project or changes billing."
  default     = ""
}

variable "region" {
  type        = string
  description = "Region for the isolated state buckets."
  default     = "southamerica-west1"
}

variable "create_state_bucket" {
  type        = bool
  description = "Explicit gate. Keep false for offline validation."
  default     = false
}

variable "state_bucket_names" {
  type        = map(string)
  description = "Three globally unique testprod-only buckets for bootstrap, GCP, and Cloudflare state."
  default     = {}
  validation {
    condition = !var.create_state_bucket || (
      length(var.state_bucket_names) == 3 &&
      contains(keys(var.state_bucket_names), "bootstrap") &&
      contains(keys(var.state_bucket_names), "gcp") &&
      contains(keys(var.state_bucket_names), "cloudflare") &&
      alltrue([for name in values(var.state_bucket_names) : can(regex("^[a-z0-9][a-z0-9._-]{1,61}[a-z0-9]$", name))]) &&
      length(toset(values(var.state_bucket_names))) == 3
    )
    error_message = "When bootstrap is enabled, provide distinct valid bucket names for bootstrap, gcp, and cloudflare state."
  }
}

variable "state_operator_members" {
  type        = map(string)
  description = "Optional principals authorized to read/write only their own state bucket. Keys: bootstrap, gcp, cloudflare. Add each grant only after its operator identity exists and is approved."
  default     = {}
  validation {
    condition = alltrue([
      for key, member in var.state_operator_members : contains(keys(var.state_bucket_names), key) && can(regex("^(user|group|serviceAccount):.+$", member))
    ])
    error_message = "State operator entries must use a configured bucket key and a valid user, group, or service account principal."
  }
}
