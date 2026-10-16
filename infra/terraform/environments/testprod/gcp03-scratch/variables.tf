variable "project_id" {
  type        = string
  description = "Explicit existing project. This isolated root never creates a project."
  default     = ""
  validation {
    condition     = !var.enable_scratch || var.project_id == "espacigo-testprod"
    error_message = "This scratch root is restricted to the explicitly reviewed existing project espacigo-testprod."
  }
}

variable "region" {
  type        = string
  description = "Region shared with the existing private test VPC."
  default     = "southamerica-west1"
  validation {
    condition     = !var.enable_scratch || var.region == "southamerica-west1"
    error_message = "This reviewed scratch root is restricted to southamerica-west1."
  }
}

variable "network_name" {
  type        = string
  description = "Existing VPC, read only. This root does not own or modify it."
  default     = "espacigo-testprod"
  validation {
    condition     = !var.enable_scratch || var.network_name == "espacigo-testprod"
    error_message = "This root is restricted to reading the pre-existing espacigo-testprod VPC."
  }
}

variable "subnetwork_name" {
  type        = string
  description = "Existing Direct VPC egress subnet, read only."
  default     = "espacigo-testprod-serverless"
  validation {
    condition     = !var.enable_scratch || var.subnetwork_name == "espacigo-testprod-serverless"
    error_message = "This root is restricted to reading the pre-existing serverless subnet."
  }
}

variable "run_id" {
  type        = string
  description = "Unique short rehearsal identifier; never reuse a scratch instance name."
  default     = ""
  validation {
    condition     = !var.enable_scratch || can(regex("^[a-z][a-z0-9-]{5,11}$", var.run_id))
    error_message = "When enabled, run_id must be 6-12 lowercase alphanumeric/hyphen characters and start with a letter."
  }
}

variable "enable_scratch" {
  type        = bool
  description = "Explicit plan/apply gate for this isolated scratch root; remains off by default."
  default     = false
}

variable "enable_job" {
  type        = bool
  description = "Separate gate for the Cloud Run migration Job; requires an uploaded digest image and active temporary secret version."
  default     = false
  validation {
    condition = !var.enable_job || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.migrator_image)) &&
      startswith(var.migrator_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_image_pushed &&
      var.confirm_scratch_secret_version &&
      var.confirm_database_owner_secret_version &&
      can(regex("^[1-9][0-9]*$", var.migrator_secret_version)) &&
      can(regex("^[1-9][0-9]*$", var.database_owner_secret_version))
    )
    error_message = "The named scratch phases require run_id pilot01, the exact scratch login/database, a sha256-pinned image, and exact numeric migrator and database-owner secret versions."
  }
}

variable "enable_observer_job" {
  type        = bool
  description = "Independent default-off gate for the fixed read-only observer Job; it preserves the already-applied five migration Jobs and IAM state."
  default     = false
  validation {
    condition = !var.enable_observer_job || (
      var.enable_scratch &&
      var.enable_job &&
      var.run_id == "pilot01" &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.observer_image)) &&
      startswith(var.observer_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_observer_image_pushed &&
      var.confirm_scratch_secret_version &&
      can(regex("^[1-9][0-9]*$", var.migrator_secret_version)) &&
      !var.allow_scratch_destroy &&
      !var.allow_replay_job_destroy
    )
    error_message = "The observer Job is restricted to the exact pilot01 scratch, existing five-job state, an uploaded digest from the existing scratch repository, and protected scratch/Jobs."
  }
}

variable "enable_owner_history_grant_job" {
  type        = bool
  description = "Independent default-off gate for the fixed migrator phase that grants postgres SELECT only on public.schema_migrations after verifying the complete history."
  default     = false
  validation {
    condition = !var.enable_owner_history_grant_job || (
      var.enable_scratch &&
      var.enable_job &&
      var.run_id == "pilot01" &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.owner_history_image)) &&
      startswith(var.owner_history_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_owner_history_image_pushed &&
      var.confirm_scratch_secret_version &&
      can(regex("^[1-9][0-9]*$", var.migrator_secret_version)) &&
      var.operator_member == "user:pasteleriahijitos@gmail.com" &&
      !var.allow_scratch_destroy &&
      !var.allow_replay_job_destroy
    )
    error_message = "The owner-history grant Job requires the exact pilot01 scratch, existing migrator secret, reviewed digest, exact operator, and protected scratch/Jobs."
  }
}

variable "enable_retired_verification_job" {
  type        = bool
  description = "Independent default-off gate for a fixed read-only owner-session verification of the retired migrator, ACLs, runtime permissions, and V1-V61 history."
  default     = false
  validation {
    condition = !var.enable_retired_verification_job || (
      var.enable_scratch && var.run_id == "pilot01" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.retired_verification_image)) &&
      startswith(var.retired_verification_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_retired_verification_image_pushed &&
      var.confirm_database_owner_secret_version &&
      can(regex("^[1-9][0-9]*$", var.database_owner_secret_version)) &&
      var.operator_member == "user:pasteleriahijitos@gmail.com" &&
      !var.allow_scratch_destroy && !var.allow_replay_job_destroy
    )
    error_message = "The retired verification Job requires the exact pilot01 scratch, uploaded digest, owner secret version, operator, and protected resources."
  }
}

variable "enable_role_probe_jobs" {
  type        = bool
  description = "Gate for the two fixed role-comment Jobs; requires an already-existing active numeric secret version and grants no credential-provisioning permissions."
  default     = false
  validation {
    condition = !var.enable_role_probe_jobs || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      !var.enable_job &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.role_probe_image)) &&
      startswith(var.role_probe_image, "${var.region}-docker.pkg.dev/${var.project_id}/gcp03-scratch-${var.run_id}/dbmigrate@sha256:") &&
      var.confirm_role_probe_image_pushed &&
      var.confirm_role_probe_secret_version &&
      can(regex("^[1-9][0-9]*$", var.role_probe_secret_version))
    )
    error_message = "Role-comment probe Jobs require the exact pilot01 target, uploaded digest, active numeric secret version, and must be planned separately from credential-provisioning permissions."
  }
}

variable "enable_role_probe_credential_permissions" {
  type        = bool
  description = "Separate gate for temporary credential provisioning/retirement IAM; Cloud SQL user operations are project-scoped and this gate requires explicit risk acknowledgment."
  default     = false
  validation {
    condition = !var.enable_role_probe_credential_permissions || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      !var.enable_job &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      var.confirm_project_scoped_cloudsql_user_ops &&
      can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.role_probe_credential_iam_expires_at))
    )
    error_message = "Credential permissions require exact pilot01 scratch target, explicit acknowledgment of project-scoped Cloud SQL user-operation risk, and a fixed expiry."
  }
}

variable "enable_replay_credential_permissions" {
  type        = bool
  description = "Independent short-lived IAM gate for creating/reconciling/retiring fixed replay credentials; remains separately controlled when migration Jobs are enabled."
  default     = false
  validation {
    condition = !var.enable_replay_credential_permissions || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      !var.enable_role_probe_jobs &&
      !var.enable_role_probe_credential_permissions &&
      !var.allow_scratch_destroy &&
      var.database_user == "espacigo_scratch_migrator" &&
      var.database_name == "espacigo_scratch" &&
      var.operator_member == "user:pasteleriahijitos@gmail.com" &&
      var.confirm_project_scoped_cloudsql_user_ops &&
      can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.replay_credential_iam_expires_at))
    )
    error_message = "Replay credential IAM requires the existing pilot01 scratch, exact operator and target logins/secrets, no role-probe or destroy gate, explicit project-scope acknowledgment, and a fixed UTC expiry. It may coexist with the separately gated migration Jobs."
  }
}

variable "role_probe_image" {
  type        = string
  description = "Digest-pinned GCP-03 image containing only fixed named phases; no mutable tags."
  default     = ""
}

variable "role_probe_secret_version" {
  type        = string
  description = "Concrete numeric Secret Manager version mounted by both probe Jobs; never use latest."
  default     = "1"
  validation {
    condition     = can(regex("^[1-9][0-9]*$", var.role_probe_secret_version))
    error_message = "The fixed role-comment probe requires an immutable numeric secret version."
  }
}

variable "confirm_role_probe_image_pushed" {
  type        = bool
  description = "Confirm only after the clean reviewed probe image digest is pushed and read back from this exact scratch repository."
  default     = false
}

variable "confirm_role_probe_secret_version" {
  type        = bool
  description = "Confirm only after the exact numeric migrator secret version exists and is verified active."
  default     = false
}

variable "confirm_project_scoped_cloudsql_user_ops" {
  type        = bool
  description = "Explicitly acknowledge that Cloud SQL users.* permissions are granted at project scope and can manage SQL users on other Cloud SQL instances in this project until expiry."
  default     = false
}

variable "role_probe_credential_iam_expires_at" {
  type        = string
  description = "Explicit RFC3339 expiry for the separate temporary credential-operator Cloud SQL/secret IAM gate."
  default     = ""
  validation {
    condition     = var.role_probe_credential_iam_expires_at == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.role_probe_credential_iam_expires_at))
    error_message = "Credential permission gate expiry must be an RFC3339 UTC timestamp."
  }
}

variable "replay_credential_iam_expires_at" {
  type        = string
  description = "Absolute UTC expiry for the separate short-lived full-replay credential IAM plan."
  default     = ""
  validation {
    condition     = var.replay_credential_iam_expires_at == "" || can(regex("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$", var.replay_credential_iam_expires_at))
    error_message = "Replay credential IAM expiry must be an RFC3339 UTC timestamp."
  }
}

variable "migrator_image" {
  type        = string
  description = "Image reference in the scratch Artifact Registry repository, pinned by immutable sha256 digest."
  default     = ""
}

variable "observer_image" {
  type        = string
  description = "Observer-only OCI reference in the existing scratch repository, pinned by immutable sha256 digest and independent from the five migration Job image."
  default     = ""
}

variable "owner_history_image" {
  type        = string
  description = "Separate digest-pinned OCI for the fixed grant_owner_history phase."
  default     = ""
}

variable "retired_verification_image" {
  type        = string
  description = "Separate digest-pinned OCI for the fixed read-only verify_retired phase."
  default     = ""
}

variable "confirm_observer_image_pushed" {
  type        = bool
  description = "Confirm only after reading back the exact observer digest from the existing scratch Artifact Registry repository."
  default     = false
}

variable "confirm_owner_history_image_pushed" {
  type        = bool
  description = "Explicit confirmation that the separate owner-history phase image digest was published and read back."
  default     = false
}

variable "confirm_retired_verification_image_pushed" {
  type        = bool
  description = "Explicit confirmation that the separate read-only retirement verification image digest was pushed and read back."
  default     = false
}

variable "confirm_project_inventory" {
  type        = bool
  description = "Set only after rechecking the exact project, VPC, subnet, and existing Cloud SQL inventory."
  default     = false
  validation {
    condition     = !var.enable_scratch || var.confirm_project_inventory
    error_message = "Scratch resources require a reviewed explicit project and inventory confirmation."
  }
}

variable "confirm_image_pushed" {
  type        = bool
  description = "Set only after the reviewed OCI manifest digest has been pushed to this exact repository and read back."
  default     = false
}

variable "confirm_scratch_secret_version" {
  type        = bool
  description = "Set only after the exact numeric migrator secret version was created outside Terraform and verified active."
  default     = false
}

variable "confirm_database_owner_secret_version" {
  type        = bool
  description = "Set only after the exact numeric database-owner secret version was created outside Terraform and verified active."
  default     = false
}

variable "migrator_secret_version" {
  type        = string
  description = "Exact active numeric Secret Manager version for bootstrap/migrate/finalize; empty until credentials are separately provisioned."
  default     = ""
  validation {
    condition     = var.migrator_secret_version == "" || can(regex("^[1-9][0-9]*$", var.migrator_secret_version))
    error_message = "When supplied, the migrator Secret Manager version must be an exact positive integer, never latest."
  }
}

variable "database_owner_secret_version" {
  type        = string
  description = "Exact active numeric Secret Manager version for database_acl/retire; empty until credentials are separately provisioned."
  default     = ""
  validation {
    condition     = var.database_owner_secret_version == "" || can(regex("^[1-9][0-9]*$", var.database_owner_secret_version))
    error_message = "When supplied, the database-owner Secret Manager version must be an exact positive integer, never latest."
  }
}

variable "allow_scratch_destroy" {
  type        = bool
  description = "Two-step scratch teardown gate; replay and role-probe Jobs have independent cleanup gates, and this gate does not lower their deletion protection."
  default     = false
}

variable "allow_role_probe_job_destroy" {
  type        = bool
  description = "Separate two-step cleanup gate for only the role-comment probe Jobs; never disables Cloud SQL or scratch protection."
  default     = false
  validation {
    condition = !var.allow_role_probe_job_destroy || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      !var.enable_job &&
      !var.allow_scratch_destroy &&
      var.confirm_role_probe_job_destroy_run_id == var.run_id
    )
    error_message = "Role-probe Job deletion requires pilot01 confirmation and must keep general scratch/Cloud SQL destruction disabled."
  }
}

variable "allow_replay_job_destroy" {
  type        = bool
  description = "Separate two-step cleanup gate for only the five replay Jobs; never disables Cloud SQL or scratch data protection."
  default     = false
  validation {
    condition = !var.allow_replay_job_destroy || (
      var.enable_scratch &&
      var.run_id == "pilot01" &&
      !var.allow_scratch_destroy &&
      !var.allow_role_probe_job_destroy &&
      var.confirm_replay_job_destroy_run_id == var.run_id
    )
    error_message = "Replay Job deletion requires exact pilot01 confirmation and must keep Cloud SQL/scratch and role-probe deletion gates disabled."
  }
}

variable "confirm_replay_job_destroy_run_id" {
  type        = string
  description = "Must exactly equal run_id when separately disabling replay Job deletion protection."
  default     = ""
}

variable "confirm_role_probe_job_destroy_run_id" {
  type        = string
  description = "Must exactly equal run_id when separately disabling role-probe Job deletion protection."
  default     = ""
}

variable "confirm_destroy_run_id" {
  type        = string
  description = "Must equal run_id for either step of scratch teardown."
  default     = ""
  validation {
    condition     = !var.allow_scratch_destroy || (var.run_id != "" && var.confirm_destroy_run_id == var.run_id)
    error_message = "Scratch deletion protection may be lowered only when confirm_destroy_run_id exactly matches run_id."
  }
}

variable "instance_tier" {
  type        = string
  description = "Reviewed scratch pairing only: N4 2 vCPU/4 GiB with Hyperdisk Balanced; recheck regional availability before a future apply."
  default     = "db-custom-N4-2-4096"
  validation {
    condition     = !var.enable_scratch || var.instance_tier == "db-custom-N4-2-4096"
    error_message = "This scratch root currently supports only the reviewed N4 2 vCPU/4 GiB tier paired with Hyperdisk Balanced."
  }
}

variable "database_name" {
  type        = string
  description = "Dedicated empty scratch database name; no restore source is configured by this root."
  default     = "espacigo_scratch"
  validation {
    condition     = var.database_name == "espacigo_scratch"
    error_message = "The review-only bootstrap is bound to the exact database name espacigo_scratch."
  }
}

variable "database_user" {
  type        = string
  description = "Temporary PostgreSQL login to be bootstrapped separately; Terraform does not create SQL users."
  default     = "espacigo_scratch_migrator"
  validation {
    condition     = can(regex("^[a-z][a-z0-9_]{0,62}$", var.database_user))
    error_message = "database_user must be a safe lowercase PostgreSQL identifier."
  }
}

variable "operator_member" {
  type        = string
  description = "Exact IAM member allowed to execute the named job and act as its scratch-only service account."
  default     = ""
  validation {
    condition     = !(var.enable_job || var.enable_observer_job || var.enable_role_probe_jobs || var.enable_role_probe_credential_permissions) || can(regex("^(user|group|serviceAccount):[^[:space:]]+$", var.operator_member))
    error_message = "Gated operations require one explicit user:, group:, or serviceAccount: operator member."
  }
}
