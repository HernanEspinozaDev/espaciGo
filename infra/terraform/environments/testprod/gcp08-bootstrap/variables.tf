variable "project_id" {
  type    = string
  default = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "GCP-08 bootstrap is restricted to the existing testprod project."
  }
}

variable "project_number" {
  type    = string
  default = "447373903153"
  validation {
    condition     = var.project_number == "447373903153"
    error_message = "GCP-08 bootstrap is restricted to the existing testprod project number."
  }
}

variable "region" {
  type    = string
  default = "southamerica-west1"
  validation {
    condition     = var.region == "southamerica-west1"
    error_message = "GCP-08 bootstrap is restricted to the Santiago region."
  }
}

variable "github_repository" {
  type    = string
  default = "HernanEspinozaDev/espaciGo"
  validation {
    condition     = var.github_repository == "HernanEspinozaDev/espaciGo"
    error_message = "GCP-08 bootstrap trusts only the EspaciGo repository."
  }
}

variable "github_repository_id" {
  type    = string
  default = "1361448198"
  validation {
    condition     = var.github_repository_id == "1361448198"
    error_message = "GCP-08 bootstrap trusts only the immutable EspaciGo repository ID."
  }
}

variable "github_repository_owner_id" {
  type    = string
  default = "176093371"
  validation {
    condition     = var.github_repository_owner_id == "176093371"
    error_message = "GCP-08 bootstrap trusts only the immutable EspaciGo repository owner ID."
  }
}

variable "enable_bootstrap" {
  type        = bool
  default     = false
  description = "Create the short-lived, restricted GCP-08 bootstrap identity. Set false and apply after the initial restore to remove its grants and trust."
}

variable "bootstrap_expires_at" {
  type        = string
  default     = ""
  description = "RFC3339 expiration for temporary IAM grants; must be in the future and no more than 24 hours away while enabled."
  validation {
    condition     = !var.enable_bootstrap || can(timecmp(var.bootstrap_expires_at, "1970-01-01T00:00:00Z"))
    error_message = "Enabled bootstrap grants require a valid RFC3339 expiration."
  }
}
