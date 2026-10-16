variable "project_id" {
  type        = string
  description = "Existing, user-selected project ID. This root never creates or bills a project."
  default     = ""
}

variable "project_number" {
  type        = string
  description = "Existing project number used by the IAP service agent and WIF."
  default     = ""
}

variable "region" {
  type        = string
  description = "Chosen test region."
  default     = "southamerica-west1"
}

variable "enable_foundation" {
  type        = bool
  description = "Explicit apply gate for APIs, network, identities, registry, buckets and secret containers."
  default     = false
  validation {
    condition     = !var.enable_foundation || (var.project_id != "" && can(regex("^[0-9]{6,20}$", var.project_number)) && var.confirm_project_inventory && var.confirm_cost_policy_review)
    error_message = "Foundation requires an existing project ID/number and explicit confirmations of isolated resource inventory and reviewed organization-policy/cost gates."
  }
}

variable "enable_database" {
  type        = bool
  description = "Separate explicit apply gate for the zonal, deletion-protected Cloud SQL instance."
  default     = false
  validation {
    condition     = !var.enable_database || var.enable_foundation
    error_message = "The SQL instance requires the separately gated foundation network and APIs."
  }
}

variable "enable_services" {
  type        = bool
  description = "Separate explicit apply gate for Cloud Run services; requires foundation and database."
  default     = false
  validation {
    condition     = !var.enable_services || (var.enable_foundation && var.enable_database && var.acknowledge_gcp04_runtime_gates && var.mailpit_private_address != "" && var.confirm_pga_iap_key_preflight && var.confirm_runtime_secret_versions && var.confirm_runtime_database_bootstrap)
    error_message = "Cloud Run services require foundation/database, GCP-04 replay/lease approval, a reviewed private Mailpit endpoint, verified IAP-key egress over Private Google Access, loaded runtime secret versions, and completed SQL runtime-user bootstrap."
  }
}

variable "enable_edge" {
  type        = bool
  description = "Separate explicit apply gate for load balancer, managed certificate and Cloud Armor."
  default     = false
  validation {
    condition     = !var.enable_edge || var.enable_services
    error_message = "The load balancer can only be enabled together with the Cloud Run services it fronts."
  }
}

variable "confirm_project_inventory" {
  type        = bool
  description = "Set only after a human reviewed the existing project inventory and confirmed this is the isolated test project."
  default     = false
}

variable "confirm_cost_policy_review" {
  type        = bool
  description = "Set only after the effective organization policies and full regional cost estimate have been reviewed before enabling foundation."
  default     = false
}

variable "confirm_pga_iap_key_preflight" {
  type        = bool
  description = "Set only after a real egress preflight confirms the private SSR can fetch https://www.gstatic.com/iap/verify/public_key through Direct VPC egress and Private Google Access."
  default     = false
}

variable "confirm_runtime_secret_versions" {
  type        = bool
  description = "Set only after all referenced Secret Manager secrets have active latest versions loaded privately and verified."
  default     = false
}

variable "confirm_runtime_database_bootstrap" {
  type        = bool
  description = "Set only after the least-privilege PostgreSQL runtime role, database grants, and connection path are created and verified outside Terraform."
  default     = false
}

variable "bucket_suffix" {
  type        = string
  description = "Globally unique suffix, supplied only after inventory confirms bucket name availability."
  default     = ""
  validation {
    condition     = !var.enable_foundation || can(regex("^[a-z0-9][a-z0-9-]{2,20}$", var.bucket_suffix))
    error_message = "When foundation is enabled, use a privately checked unique lowercase bucket suffix."
  }
}

variable "github_repository" {
  type        = string
  description = "Exact owner/repository allowed by GitHub Workload Identity Federation."
  default     = "HernanEspinozaDev/espaciGo"
}

variable "serverless_subnet_cidr" {
  type        = string
  description = "Dedicated non-overlapping subnet for Cloud Run Direct VPC egress."
  default     = "10.80.0.0/24"
}

variable "cloudsql_tier" {
  type        = string
  description = "Explicit Enterprise N4 custom machine type; current candidate is 2 vCPU/4 GiB."
  default     = "db-custom-N4-2-4096"
}

variable "cloudsql_disk_gib" {
  type        = number
  description = "Initial Hyperdisk Balanced size; minimum is 20 GiB."
  default     = 20
}

variable "cloudsql_disk_limit_gib" {
  type        = number
  description = "Hard cap for automatic disk increase."
  default     = 100
}

variable "api_service_name" {
  type    = string
  default = "espacigo-test-api"
}

variable "public_service_name" {
  type    = string
  default = "espacigo-test-public"
}

variable "private_service_name" {
  type    = string
  default = "espacigo-test-private"
}

variable "api_image" {
  type        = string
  description = "Existing API image by sha256 digest; CI owns build and upload."
  default     = ""
  validation {
    condition     = !var.enable_services || can(regex("@sha256:[0-9a-f]{64}$", var.api_image))
    error_message = "When services are enabled, api_image must be an existing sha256-digest image reference."
  }
}

variable "frontend_image" {
  type        = string
  description = "Existing Angular SSR image by sha256 digest; CI owns build and upload."
  default     = ""
  validation {
    condition     = !var.enable_services || can(regex("@sha256:[0-9a-f]{64}$", var.frontend_image))
    error_message = "When services are enabled, frontend_image must be an existing sha256-digest image reference."
  }
}

variable "public_hostname" {
  type    = string
  default = "espacigo.testingpage.store"
}

variable "private_hostname" {
  type    = string
  default = "espacigo-demo.testingpage.store"
}

variable "api_max_instances" {
  type        = number
  description = "API hard cap; do not raise before shared replay and worker leases are implemented."
  default     = 1
  validation {
    condition     = var.api_max_instances == 1
    error_message = "Keep API max instances at one until shared replay and worker lease gates are resolved."
  }
}

variable "web_max_instances" {
  type        = number
  description = "SSR service hard cap for the initial test environment."
  default     = 2
}

variable "public_catalog_requests_per_minute" {
  type        = number
  description = "Per-client-IP Cloud Armor threshold for the public catalog listing route."
  default     = 60
  validation {
    condition     = var.public_catalog_requests_per_minute >= 1 && var.public_catalog_requests_per_minute <= 600
    error_message = "Catalog rate limit must be between 1 and 600 requests per minute per client IP."
  }
}

variable "public_other_requests_per_minute" {
  type        = number
  description = "Per-client-IP Cloud Armor threshold shared by other public SSR/API/image/sitemap/readiness paths."
  default     = 120
  validation {
    condition     = var.public_other_requests_per_minute >= 1 && var.public_other_requests_per_minute <= 1200
    error_message = "Other public route rate limit must be between 1 and 1200 requests per minute per client IP."
  }
}

variable "iap_evaluator_members" {
  type        = list(string)
  description = "Explicit IAP principals. Empty by default; no public or organization-wide binding is added."
  default     = []
}

variable "mailpit_private_address" {
  type        = string
  description = "Private Mailpit SMTP host:port from the separately reviewed GCP-05 VM. It is not provisioned here."
  default     = ""
  validation {
    condition     = var.mailpit_private_address == "" || can(regex("^[a-zA-Z0-9.-]+:[0-9]{1,5}$", var.mailpit_private_address))
    error_message = "When supplied, the private Mailpit SMTP endpoint must be a host:port pair."
  }
}

variable "acknowledge_gcp04_runtime_gates" {
  type        = bool
  description = "Must remain false until shared replay and worker leases are implemented and accepted in GCP-04."
  default     = false
}
