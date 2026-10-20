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
    condition     = !var.enable_services || (var.enable_foundation && (var.enable_database || var.api_gcp06_candidate_run_id != "") && (var.mailpit_private_address != "" || var.enable_mailpit) && var.confirm_runtime_secret_versions && var.confirm_runtime_database_bootstrap)
    error_message = "Cloud Run services require foundation, either the managed database or the exact GCP-06 candidate, a private Mailpit endpoint or gated VM, loaded runtime secret versions, and completed SQL runtime-user bootstrap."
  }
}

variable "enable_edge" {
  type        = bool
  description = "Separate explicit apply gate for load balancer, managed certificate and Cloud Armor."
  default     = false
  validation {
    condition     = !var.enable_edge || (var.enable_services && var.confirm_pga_iap_key_preflight)
    error_message = "The edge requires Cloud Run services and a verified IAP-key egress preflight through Direct VPC and Private Google Access."
  }
}

variable "enable_mailpit" {
  type        = bool
  description = "Separate gate for a private, single-VM Mailpit test sink with IAP TCP access to its UI."
  default     = false
  validation {
    condition     = !var.enable_mailpit || var.enable_foundation
    error_message = "The private Mailpit VM requires the separately gated foundation network and APIs."
  }
  validation {
    condition     = !var.enable_mailpit || var.mailpit_private_address == ""
    error_message = "When enable_mailpit is true, mailpit_private_address is derived from the VM's reserved internal IP."
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
  description = "Required only when enable_edge routes all SSR egress through Direct VPC; set after verifying access to the IAP key endpoint through Private Google Access."
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

variable "github_repository_id" {
  type        = string
  description = "Immutable GitHub repository ID allowed by Workload Identity Federation."
  default     = "1361448198"
}

variable "github_repository_owner_id" {
  type        = string
  description = "Immutable GitHub repository owner ID allowed by Workload Identity Federation."
  default     = "176093371"
}

variable "terraform_state_bucket" {
  type        = string
  description = "Existing canonical GCS backend bucket for the testprod/gcp Terraform state."
  default     = "espacigo-testprod-tfstate-447373903153-gcp"
}

variable "serverless_subnet_cidr" {
  type        = string
  description = "Dedicated non-overlapping subnet for Cloud Run Direct VPC egress."
  default     = "10.80.0.0/24"
}

variable "api_serverless_subnet_cidr" {
  type        = string
  description = "Dedicated API Direct VPC subnet so only API workloads can reach the private Mailpit SMTP port."
  default     = "10.81.0.0/26"
  validation {
    condition     = can(cidrhost(var.api_serverless_subnet_cidr, 1))
    error_message = "api_serverless_subnet_cidr must be a valid IPv4 CIDR range."
  }
}

variable "mailpit_subnet_cidr" {
  type        = string
  description = "Dedicated private subnet for the Mailpit test VM."
  default     = "10.81.1.0/28"
  validation {
    condition     = can(cidrhost(var.mailpit_subnet_cidr, 1))
    error_message = "mailpit_subnet_cidr must be a valid IPv4 CIDR range."
  }
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

variable "api_gcp06_candidate_run_id" {
  type        = string
  default     = ""
  description = "When set, direct only the API test service to the reviewed run-scoped GCP-06 candidate. Empty preserves the existing database target."
  validation {
    condition     = var.api_gcp06_candidate_run_id == "" || (var.api_gcp06_candidate_run_id == "20261020-gcp06a" && var.project_id == "espacigo-testprod" && var.region == "southamerica-west1")
    error_message = "Only candidate 20261020-gcp06a in the existing testprod Santiago project is allowed by this cut."
  }
}

variable "api_gcp06_candidate_password_version" {
  type        = string
  default     = ""
  description = "Numeric, pre-created Secret Manager version for the candidate API database login; Terraform never reads the secret payload."
  validation {
    condition     = var.api_gcp06_candidate_run_id == "" || can(regex("^[1-9][0-9]*$", var.api_gcp06_candidate_password_version))
    error_message = "Candidate mode requires an explicit positive Secret Manager version number."
  }
}

variable "enable_gcp06_admin_bootstrap_job" {
  type        = bool
  default     = false
  description = "Prepare the dedicated one-shot Cloud Run Job for the exact candidate global-admin bootstrap."
}

variable "enable_gcp06_test_access_job" {
  type        = bool
  default     = false
  description = "Prepare the dedicated one-shot candidate test-account access Job. Defaults to preview mode; apply mode requires a separately reviewed exact cohort."
}

variable "enable_gcp06_sql_inspector_job" {
  type        = bool
  default     = false
  description = "Prepare the dedicated, private, read-only GCP-06 SQL catalog inspector Job."
}

variable "gcp06_sql_inspector_image" {
  type        = string
  default     = ""
  description = "Immutable Artifact Registry digest for the catalog-only inspector Job."
}

variable "gcp06_sql_inspector_secret_version" {
  type        = string
  default     = ""
  description = "Exact numeric Secret Manager version containing the one-run inspector login credential."
}

variable "gcp06_sql_inspector_operator_member" {
  type        = string
  default     = ""
  description = "Single nominated human principal permitted to invoke only the SQL inspector Job."
  validation {
    condition     = var.gcp06_sql_inspector_operator_member == "" || can(regex("^(user|serviceAccount):[A-Za-z0-9._@+-]+$", var.gcp06_sql_inspector_operator_member))
    error_message = "Inspector Job invoker must be one explicit user or service-account principal."
  }
}

variable "gcp06_sql_inspector_iam_expiry" {
  type        = string
  default     = ""
  description = "RFC3339 UTC expiration for the inspector's instance-scoped Cloud SQL client grant."
}

variable "enable_gcp06_jobs_reviewed" {
  type        = bool
  default     = false
  description = "Human review gate for creating candidate-only Jobs and their exact service-account, SQL, Artifact Registry, and Secret Manager permissions."
  validation {
    condition     = !var.enable_gcp06_jobs_reviewed || (var.enable_foundation && var.project_id == "espacigo-testprod" && var.region == "southamerica-west1" && ((var.api_gcp06_candidate_run_id == "20261020-gcp06a" && (var.enable_gcp06_admin_bootstrap_job || var.enable_gcp06_test_access_job)) || (var.api_gcp06_candidate_run_id == "" && var.enable_gcp06_sql_inspector_job && !var.enable_gcp06_admin_bootstrap_job && !var.enable_gcp06_test_access_job)))
    error_message = "GCP-06 Jobs require the reviewed foundation, exact candidate, test project/region, and an explicitly selected operation; the inspector may be planned independently from the API cutover."
  }
}

variable "gcp06_admin_bootstrap_image" {
  type        = string
  default     = ""
  description = "Immutable Artifact Registry digest for the separately reviewed owner-bootstrap Job."
}

variable "gcp06_activation_login" {
  type        = string
  default     = ""
  description = "One-time PostgreSQL login that is SET ROLE member only of espacigo_gadm_bootstrap."
  validation {
    condition     = var.gcp06_activation_login == "" || can(regex("^gadm_activation_[a-f0-9]{12}$", var.gcp06_activation_login))
    error_message = "GCP-06 activation login must be one-time and match the restricted login contract."
  }
}

variable "gcp06_admin_bootstrap_password_version" {
  type        = string
  default     = ""
  description = "Numeric Secret Manager version for the one-time activation login password; the secret payload is never stored in Terraform."
}

variable "gcp06_bootstrap_operator_member" {
  type        = string
  default     = ""
  description = "One nominated human principal permitted to invoke only the candidate admin-bootstrap Job."
  validation {
    condition     = var.gcp06_bootstrap_operator_member == "" || can(regex("^(user|serviceAccount):[A-Za-z0-9._@+-]+$", var.gcp06_bootstrap_operator_member))
    error_message = "Bootstrap Job invoker must be one explicit user or service-account principal."
  }
}

variable "gcp06_test_access_image" {
  type        = string
  default     = ""
  description = "Immutable Artifact Registry digest for the candidate test-access Job."
}

variable "gcp06_test_access_login" {
  type        = string
  default     = ""
  description = "One-time PostgreSQL login that is SET ROLE member only of espacigo_gcp06_test_access."
  validation {
    condition     = var.gcp06_test_access_login == "" || can(regex("^gcp06_test_access_[a-f0-9]{12}$", var.gcp06_test_access_login))
    error_message = "Test-access login must be one-time and match the restricted login contract."
  }
}

variable "gcp06_test_access_db_password_version" {
  type        = string
  default     = ""
  description = "Numeric Secret Manager version for the separate one-time SQL test-access login."
}

variable "gcp06_test_app_password_version" {
  type        = string
  default     = ""
  description = "Numeric Secret Manager version for the shared synthetic application password; Secret Manager only, never Terraform payload/state."
}

variable "gcp06_test_access_operator_member" {
  type        = string
  default     = ""
  description = "One nominated human principal permitted to invoke only the candidate test-access Job."
  validation {
    condition     = var.gcp06_test_access_operator_member == "" || can(regex("^(user|serviceAccount):[A-Za-z0-9._@+-]+$", var.gcp06_test_access_operator_member))
    error_message = "Test-access Job invoker must be one explicit user or service-account principal."
  }
}

variable "gcp06_test_access_mode" {
  type        = string
  default     = "preview"
  description = "Preview emits only count and target-set digest. Apply changes the exact reviewed candidate cohort and revokes its sessions/tokens."
  validation {
    condition     = contains(["preview", "apply"], var.gcp06_test_access_mode)
    error_message = "GCP-06 test-access Job mode must be preview or apply."
  }
}

variable "gcp06_test_access_expected_count" {
  type        = number
  default     = 0
  description = "Exact user count from the separately reviewed, non-PII preview. Required only in apply mode."
}

variable "gcp06_test_access_expected_sha256" {
  type        = string
  default     = ""
  description = "Order-independent digest of the candidate UUID cohort from the reviewed preview; no account IDs are stored in Terraform."
}

variable "gcp06_test_access_operation_id" {
  type        = string
  default     = ""
  description = "Unique UUID for one apply attempt. Reconcile this audit ID after uncertain outcomes; never re-run with a new ID until reconciled."
}

variable "gcp06_test_access_confirmation" {
  type        = string
  default     = ""
  description = "Exact candidate/count/digest/operation confirmation required by the apply Job. Contains no secret."
}

variable "api_contract_encryption_key_version" {
  type        = string
  default     = "latest"
  description = "Numeric Secret Manager version mounted by the API; before enabling services, verify this exact version is ENABLED through metadata without reading its payload."
  validation {
    condition     = !var.enable_services || can(regex("^[1-9][0-9]*$", var.api_contract_encryption_key_version))
    error_message = "When API services are enabled, pin a positive numeric contract key version."
  }
}

variable "api_private_object_prefix" {
  type        = string
  default     = ""
  description = "Private Cloud Storage object prefix configured for the API; the G03O transfer uses g03o-1017."
  validation {
    condition     = contains(["", "g03o-1017"], var.api_private_object_prefix)
    error_message = "The API private object prefix must be empty or the reviewed g03o-1017 namespace."
  }
}

variable "api_privacy_replay_prefix" {
  type        = string
  default     = ""
  description = "Private replay registry object prefix configured for the API; the G03O transfer uses g03o-1017/privacy-replay."
  validation {
    condition     = contains(["", "g03o-1017/privacy-replay"], var.api_privacy_replay_prefix)
    error_message = "The API privacy replay prefix must be empty or the reviewed g03o-1017/privacy-replay namespace."
  }
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

variable "private_demo_origin_override" {
  type        = string
  description = "Optional private Cloud Run run.app origin for the initial no-DNS test; set from private_web_runapp_url after the first service apply."
  default     = ""
  validation {
    condition     = var.private_demo_origin_override == "" || can(regex("^https://[a-z0-9.-]+\\.run\\.app$", var.private_demo_origin_override))
    error_message = "private_demo_origin_override must be empty or an HTTPS Cloud Run run.app origin without a path."
  }
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
  description = "SSR service hard cap for the constrained single-instance synthetic test stage."
  default     = 1
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
  description = "Existing private Mailpit SMTP host:port when enable_mailpit is false; otherwise the address is derived from the test VM."
  default     = ""
  validation {
    condition     = var.mailpit_private_address == "" || can(regex("^[a-zA-Z0-9.-]+:[0-9]{1,5}$", var.mailpit_private_address))
    error_message = "When supplied, the private Mailpit SMTP endpoint must be a host:port pair."
  }
}

variable "mailpit_cos_image" {
  type        = string
  description = "Optional fully qualified COS image URL for the Mailpit VM; empty keeps the cos-stable family lookup."
  default     = ""
  validation {
    condition     = var.mailpit_cos_image == "" || can(regex("^https://www\\.googleapis\\.com/compute/v1/projects/cos-cloud/global/images/cos-stable-[a-z0-9-]+$", var.mailpit_cos_image))
    error_message = "mailpit_cos_image must be empty or a fully qualified cos-cloud cos-stable image URL."
  }
}

variable "mailpit_zone" {
  type        = string
  description = "Explicit zone for the small, private Mailpit test VM and persistent data disk."
  default     = "southamerica-west1-a"
  validation {
    condition     = startswith(var.mailpit_zone, "southamerica-west1-")
    error_message = "mailpit_zone must be a zone in southamerica-west1."
  }
}

variable "acknowledge_gcp04_runtime_gates" {
  type        = bool
  description = "Reserved for full GCP-04 operational acceptance; leaving this false for the single-instance synthetic test does not imply that acceptance."
  default     = false
}
