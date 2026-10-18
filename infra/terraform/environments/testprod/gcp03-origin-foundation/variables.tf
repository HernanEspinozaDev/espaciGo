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

variable "enable_origin_bootstrap_job" {
  type        = bool
  default     = false
  description = "Separate default-off gate for the one-shot origin bootstrap Job. It creates no secret version and does not execute the Job."
  validation {
    condition     = !var.enable_origin_bootstrap_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && can(regex("@sha256:[0-9a-f]{64}$", var.origin_executor_image)) && can(regex("^[1-9][0-9]*$", var.admin_password_secret_version)) && var.confirm_admin_password_secret_version)
    error_message = "Origin bootstrap requires the exact g03o-1017 Foundation, digest-pinned executor, and reviewed numeric admin-password version."
  }
}

variable "enable_origin_observer_job" {
  type        = bool
  default     = false
  description = "Separate default-off gate for the IAM-only pre-restore observer; requires bootstrap to be completed and reviewed first."
  validation {
    condition     = !var.enable_origin_observer_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && var.confirm_origin_bootstrap_completed && can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256)) && can(regex("@sha256:[0-9a-f]{64}$", var.origin_executor_image)))
    error_message = "Origin observer requires the exact g03o-1017 Foundation, a reviewed digest, and hash-bound evidence that bootstrap completed."
  }
}

variable "enable_origin_membership_job" {
  type        = bool
  default     = false
  description = "Default-off one-shot transaction that grants only SET membership on the existing runtime role to the fixed postgres login. It does not rerun bootstrap or create roles."
  validation {
    condition     = !var.enable_origin_membership_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && can(regex("@sha256:[0-9a-f]{64}$", var.origin_membership_image)) && can(regex("^[1-9][0-9]*$", var.admin_password_secret_version)) && var.confirm_admin_password_secret_version)
    error_message = "Runtime membership remediation requires the exact g03o-1017 target, digest-pinned executor, and reviewed numeric admin-password version."
  }
}

variable "origin_membership_image" {
  type        = string
  default     = ""
  description = "Digest-pinned executor image containing the one-shot runtime membership remediation; separate from the existing bootstrap image."
  validation {
    condition     = var.origin_membership_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_membership_image)) && startswith(var.origin_membership_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:"))
    error_message = "Membership executor image must be digest-pinned in the reviewed GCP-03 Artifact Registry repository."
  }
}

variable "enable_origin_membership_diagnostic_job" {
  type        = bool
  default     = false
  description = "Default-off one-shot read-only catalog diagnostic for the fixed postgres/runtime membership state."
  validation {
    condition     = !var.enable_origin_membership_diagnostic_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && can(regex("@sha256:[0-9a-f]{64}$", var.origin_membership_diagnostic_image)) && can(regex("^[1-9][0-9]*$", var.admin_password_secret_version)) && var.confirm_admin_password_secret_version)
    error_message = "Membership diagnostic requires the exact g03o-1017 target, a digest-pinned executor, and a reviewed numeric admin-password version."
  }
}

variable "origin_membership_diagnostic_image" {
  type        = string
  default     = ""
  description = "Digest-pinned gcp03origin executor containing the read-only membership diagnostic phase."
  validation {
    condition     = var.origin_membership_diagnostic_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_membership_diagnostic_image)) && startswith(var.origin_membership_diagnostic_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:"))
    error_message = "Membership diagnostic image must be digest-pinned in the reviewed GCP-03 executor repository."
  }
}

variable "enable_origin_owner_acl_diagnostic_job" {
  type        = bool
  default     = false
  description = "Default-off read-only owner/ACL and import-count diagnostic for the fixed origin target."
  validation {
    condition     = !var.enable_origin_owner_acl_diagnostic_job || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && can(regex("@sha256:[0-9a-f]{64}$", var.origin_owner_acl_diagnostic_image)) && var.admin_password_secret_version == "1" && var.confirm_admin_password_secret_version)
    error_message = "Owner/ACL diagnostic requires the exact target, digest-pinned executor, and confirmed admin-password secret version 1."
  }
}

variable "origin_owner_acl_diagnostic_image" {
  type        = string
  default     = ""
  description = "Digest-pinned GCP-03 origin executor with the fixed read-only owner/ACL diagnostic profile."
  validation {
    condition     = var.origin_owner_acl_diagnostic_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_owner_acl_diagnostic_image)) && startswith(var.origin_owner_acl_diagnostic_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:"))
    error_message = "Owner/ACL diagnostic image must be digest-pinned in the reviewed origin executor repository."
  }
}

variable "enable_uploader_impersonation_binding" {
  type        = bool
  default     = false
  description = "Default-off temporary run-scoped getAccessToken custom role and binding for the human uploader operator; destroy both after the upload/readback phase."
  validation {
    condition     = !var.enable_uploader_impersonation_binding || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && var.enable_origin_ciphertext_uploader)
    error_message = "Uploader impersonation binding requires the explicitly confirmed g03o-1017 Foundation and the run-scoped uploader identity."
  }
}

variable "enable_origin_import_infrastructure" {
  type        = bool
  default     = false
  description = "Default-off gate for private import storage, K_run Secret Manager metadata, and narrowly scoped IAM; creates no secret version and no Job."
  validation {
    condition     = !var.enable_origin_import_infrastructure || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017")
    error_message = "Import infrastructure requires the confirmed g03o-1017 foundation."
  }
}

variable "enable_origin_import_job" {
  type        = bool
  default     = false
  description = "Default-off gate for the one-shot hash-bound import Job and its run-prefix-only private file writer permission."
  validation {
    condition     = !var.enable_origin_import_job || (var.enable_origin_import_infrastructure && var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" && var.confirm_origin_bootstrap_completed && can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256)) && var.confirm_admin_password_secret_version && can(regex("^[1-9][0-9]*$", var.admin_password_secret_version)) && can(regex("@sha256:[0-9a-f]{64}$", var.origin_import_image)) && can(regex("@sha256:[0-9a-f]{64}$", var.origin_proxy_image)) && can(regex("^[1-9][0-9]*$", var.transport_key_secret_version)) && var.confirm_transport_key_secret_version && can(regex("^[0-9a-f]{64}$", var.import_receipt_sha256)) && can(regex("^[0-9a-f]{64}$", var.import_manifest_sha256)) && can(regex("^[0-9a-f]{64}$", var.import_observer_sha256)) && can(regex("^[0-9a-f]{64}$", var.import_normalization_sha256)) && var.import_receipt_object == "runs/g03o-1017/receipt.json" && can(regex("^[1-9][0-9]*$", var.import_receipt_generation)) && var.import_receipt_bytes > 0)
    error_message = "Origin import requires the separately enabled import infrastructure, exact run evidence, numeric reviewed secrets, and digest-pinned executor/proxy images."
  }
}

variable "enable_origin_postrestore_job" {
  type        = bool
  default     = false
  description = "Default-off one-shot verifier, gated on the exact completed import receipt and generation-bound review artifacts."
  validation {
    condition = !var.enable_origin_postrestore_job || (
      var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017" &&
      var.confirm_origin_bootstrap_completed && var.confirm_origin_import_completed &&
      can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256)) &&
      can(regex("^[0-9a-f]{64}$", var.origin_import_evidence_sha256)) &&
      can(regex("^[0-9a-f]{64}$", var.import_receipt_sha256)) &&
      var.import_manifest_sha256 == "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30" &&
      var.import_toc_sha256 == "0677025fc73b036b9da9804ec457a8beea737662c9cf8537ba1bddd377b9b942" &&
      var.import_normalization_sha256 == "2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2" &&
      var.import_migration_sha256 == "eca6b4295b593bc3ea3af470fb747bd6e788e141df4a57c8a0fae91ea6ad1fb2" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.origin_import_image)) &&
      can(regex("@sha256:[0-9a-f]{64}$", var.origin_postrestore_image)) &&
      var.origin_postrestore_image == var.origin_import_image &&
      can(regex("@sha256:[0-9a-f]{64}$", var.origin_proxy_image)) &&
      var.import_receipt_object == "runs/g03o-1017/receipt.json" &&
      can(regex("^[1-9][0-9]*$", var.import_receipt_generation)) && var.import_receipt_bytes > 0 &&
      var.import_toc_object == "runs/g03o-1017/toc.list" &&
      var.import_normalization_object == "runs/g03o-1017/normalization.sql" &&
      can(regex("^[1-9][0-9]*$", var.import_toc_generation)) && var.import_toc_bytes > 0 &&
      can(regex("^[1-9][0-9]*$", var.import_normalization_generation)) && var.import_normalization_bytes > 0
    )
    error_message = "Post-restore verifier requires exact g03o-1017 import completion, receipt, object generations, hashes, and pinned images."
  }
}

variable "enable_origin_recovery_plan_job" {
  type        = bool
  default     = false
  description = "Default-off one-shot read-only recovery planner over the observed g03o-1017 restored copy; reads exact U2 generations and writes no objects."
  validation {
    condition = !var.enable_origin_recovery_plan_job || (
      var.enable_origin_import_infrastructure && var.enable_foundation && var.confirm_origin_rehearsal &&
      var.run_id == "g03o-1017" && var.confirm_origin_bootstrap_completed &&
      var.confirm_origin_restored_copy_observed && var.origin_recovery_import_failure_sha256 == "bd4054e1a49eb082467f3c88bba5ef27d3aaec972115962e86e6ab87cb04f349" &&
      var.confirm_admin_password_secret_version &&
      can(regex("^[1-9][0-9]*$", var.admin_password_secret_version)) && var.admin_password_secret_version == "1" &&
      var.confirm_transport_key_secret_version && can(regex("^[1-9][0-9]*$", var.transport_key_secret_version)) && var.transport_key_secret_version == "1" &&
      can(regex("@sha256:[0-9a-f]{64}$", var.origin_recovery_image)) &&
      can(regex("@sha256:[0-9a-f]{64}$", var.origin_proxy_image)) &&
      var.import_receipt_object == "runs/g03o-1017/receipt.json" &&
      can(regex("^[1-9][0-9]*$", var.import_receipt_generation)) &&
      var.import_receipt_generation == "1792273080133875" &&
      var.import_receipt_bytes == 1892 &&
      var.import_receipt_sha256 == "42f817e89382af2df3e0f452b594311936e91839fd92cc251f133bd4d867377c" &&
      var.import_manifest_sha256 == "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30" &&
      can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256)) &&
      can(regex("^[0-9a-f]{64}$", var.import_observer_sha256)) &&
      var.import_normalization_sha256 == "2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2" &&
      var.origin_recovery_diagnostic_sha256 == "3325288140b1f2de8bb944f7c4c1599fcf56f1a19531dc334bf171629f99391a"
    )
    error_message = "Recovery planning requires the observed restored copy, exact failed-import and read-only diagnostic evidence, reviewed U2 generations/hashes, numeric existing secrets, and digest-pinned images."
  }
}

variable "confirm_origin_restored_copy_observed" {
  type        = bool
  default     = false
  description = "Acknowledges the successful read-only diagnostic of the existing restored copy before recovery planning; does not assert a successful import."
}

variable "origin_recovery_import_failure_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of the fixed sanitized failed-import report that identifies the existing restored copy requiring recovery."
  validation {
    condition     = var.origin_recovery_import_failure_sha256 == "" || var.origin_recovery_import_failure_sha256 == "bd4054e1a49eb082467f3c88bba5ef27d3aaec972115962e86e6ab87cb04f349"
    error_message = "Recovery must bind the exact preserved F4 failed-import report SHA-256."
  }
}

variable "enable_origin_recovery_apply_job" {
  type        = bool
  default     = false
  description = "Default-off mutating recovery Job; requires a separately reviewed SHA-256 from the recovery plan report."
  validation {
    condition = !var.enable_origin_recovery_apply_job || (
      var.enable_origin_recovery_plan_job && var.confirm_origin_recovery_plan_reviewed &&
      can(regex("^[0-9a-f]{64}$", var.origin_recovery_approved_plan_sha256))
    )
    error_message = "Recovery apply requires the plan Job gate and explicit review of its exact 64-character plan SHA-256."
  }
}

variable "confirm_origin_recovery_plan_reviewed" {
  type        = bool
  default     = false
  description = "Acknowledges a human review of the exact current recovery plan SHA-256."
}

variable "origin_recovery_approved_plan_sha256" {
  type        = string
  default     = ""
  description = "Exact recovery plan SHA-256 approved for one-shot apply; no plan is applied by Terraform."
  validation {
    condition     = var.origin_recovery_approved_plan_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.origin_recovery_approved_plan_sha256))
    error_message = "Approved recovery plan hash must be a lowercase SHA-256 digest."
  }
}

variable "origin_recovery_diagnostic_sha256" {
  type        = string
  default     = "3325288140b1f2de8bb944f7c4c1599fcf56f1a19531dc334bf171629f99391a"
  description = "Fixed hash of the successful, read-only owner/ACL diagnostic report used as recovery evidence."
}

variable "origin_recovery_image" {
  type        = string
  default     = ""
  description = "Digest-pinned recovery entrypoint image; distinct from the importer image tag and invoked with an explicit recovery mode."
  validation {
    condition     = var.origin_recovery_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_recovery_image)) && startswith(var.origin_recovery_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-recovery@sha256:"))
    error_message = "Recovery image must be digest-pinned under the existing reviewed GCP-03 Artifact Registry repository path."
  }
}

variable "confirm_origin_import_completed" {
  type        = bool
  default     = false
  description = "Acknowledges a completed, independently reviewed import receipt before enabling verification."
}

variable "origin_import_evidence_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of the sanitized successful import receipt/report."
  validation {
    condition     = var.origin_import_evidence_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.origin_import_evidence_sha256))
    error_message = "Import evidence must be identified by lowercase SHA-256."
  }
}

variable "origin_postrestore_image" {
  type        = string
  default     = ""
  description = "Digest-pinned Python/PostgreSQL image containing the read-only post-restore verifier."
  validation {
    condition     = var.origin_postrestore_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_postrestore_image)) && startswith(var.origin_postrestore_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-import@sha256:"))
    error_message = "Post-restore verifier image must be digest-pinned in the existing GCP-03 repository."
  }
}

variable "import_toc_object" {
  type    = string
  default = ""
}
variable "import_toc_generation" {
  type    = string
  default = ""
}
variable "import_toc_bytes" {
  type    = number
  default = 0
}
variable "import_toc_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.import_toc_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.import_toc_sha256))
    error_message = "TOC allowlist must be identified by lowercase SHA-256."
  }
}
variable "import_normalization_object" {
  type    = string
  default = ""
}
variable "import_normalization_generation" {
  type    = string
  default = ""
}
variable "import_normalization_bytes" {
  type    = number
  default = 0
}
variable "import_migration_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.import_migration_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.import_migration_sha256))
    error_message = "Migration inventory must be identified by lowercase SHA-256."
  }
}

variable "enable_origin_ciphertext_uploader" {
  type        = bool
  default     = false
  description = "Separate default-off gate for a run-scoped create-only uploader identity; this identity receives no secrets or Cloud SQL access."
  validation {
    condition     = !var.enable_origin_ciphertext_uploader || (var.enable_foundation && var.confirm_origin_rehearsal && var.run_id == "g03o-1017")
    error_message = "Ciphertext uploader requires the explicitly confirmed g03o-1017 Foundation."
  }
}

variable "import_receipt_object" {
  type    = string
  default = ""
}

variable "import_receipt_generation" {
  type    = string
  default = ""
}

variable "import_receipt_bytes" {
  type    = number
  default = 0
}

variable "origin_executor_image" {
  type        = string
  default     = ""
  description = "Immutable SHA-256 image digest for the separate gcp03origin executor, stored in the existing scratch Artifact Registry repository."
  validation {
    condition     = var.origin_executor_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_executor_image)) && startswith(var.origin_executor_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin@sha256:"))
    error_message = "Origin executor image must be a SHA-256 digest in the existing reviewed GCP-03 Artifact Registry repository."
  }
}

variable "origin_proxy_image" {
  type        = string
  default     = ""
  description = "Immutable SHA-256 digest for Cloud SQL Auth Proxy, required only by the import Job."
  validation {
    condition     = var.origin_proxy_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_proxy_image)) && startswith(var.origin_proxy_image, "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:"))
    error_message = "Cloud SQL Auth Proxy image must be pinned by SHA-256 in the official Google registry."
  }
}

variable "origin_import_image" {
  type        = string
  default     = ""
  description = "Immutable SHA-256 digest for the Python/PostgreSQL one-shot origin importer image."
  validation {
    condition     = var.origin_import_image == "" || (can(regex("@sha256:[0-9a-f]{64}$", var.origin_import_image)) && startswith(var.origin_import_image, "southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03origin-import@sha256:"))
    error_message = "Origin import image must be a SHA-256 digest in the existing reviewed GCP-03 Artifact Registry repository."
  }
}

variable "transport_key_secret_version" {
  type        = string
  default     = ""
  description = "Exact numeric K_run Secret Manager version; secret payload and version are never Terraform-managed."
  validation {
    condition     = var.transport_key_secret_version == "" || can(regex("^[1-9][0-9]*$", var.transport_key_secret_version))
    error_message = "K_run secret version must be a positive numeric version."
  }
}

variable "confirm_transport_key_secret_version" {
  type        = bool
  default     = false
  description = "Acknowledges a separately provisioned and reviewed exact K_run version."
}

variable "import_receipt_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.import_receipt_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.import_receipt_sha256))
    error_message = "Receipt must be identified by lowercase SHA-256."
  }
}

variable "import_manifest_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.import_manifest_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.import_manifest_sha256))
    error_message = "Manifest must be identified by lowercase SHA-256."
  }
}

variable "import_observer_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.import_observer_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.import_observer_sha256))
    error_message = "Observer evidence must be identified by lowercase SHA-256."
  }
}

variable "import_normalization_sha256" {
  type    = string
  default = ""
  validation {
    condition     = var.import_normalization_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.import_normalization_sha256))
    error_message = "Normalization SQL must be identified by lowercase SHA-256."
  }
}

variable "admin_password_secret_version" {
  type        = string
  default     = ""
  description = "Exact numeric Secret Manager version for a future bootstrap plan; never use latest."
  validation {
    condition     = var.admin_password_secret_version == "" || can(regex("^[1-9][0-9]*$", var.admin_password_secret_version))
    error_message = "The bootstrap password secret version must be a positive numeric version."
  }
}

variable "confirm_admin_password_secret_version" {
  type        = bool
  default     = false
  description = "Acknowledges that the exact active admin-password version was provisioned out of band and reviewed."
}

variable "confirm_origin_bootstrap_completed" {
  type        = bool
  default     = false
  description = "Acknowledges successful bootstrap evidence before creating the observer IAM database user and Job."
}

variable "bootstrap_evidence_sha256" {
  type        = string
  default     = ""
  description = "SHA-256 of the reviewed sanitized bootstrap report required before creating the observer IAM user/Job."
  validation {
    condition     = var.bootstrap_evidence_sha256 == "" || can(regex("^[0-9a-f]{64}$", var.bootstrap_evidence_sha256))
    error_message = "Bootstrap evidence must be identified by a lowercase SHA-256 digest."
  }
}

locals {
  foundation_enabled             = var.enable_foundation && var.confirm_origin_rehearsal
  prefix                         = "espacigo-gcp03-origin-${var.run_id}"
  admin_account_id               = "g03o-admin-${var.run_id}"
  instance_name                  = "${local.prefix}-pg"
  admin_enabled                  = local.foundation_enabled && var.enable_origin_bootstrap_job
  membership_enabled             = local.foundation_enabled && var.enable_origin_membership_job
  membership_diagnostic_enabled  = local.foundation_enabled && var.enable_origin_membership_diagnostic_job
  owner_acl_diagnostic_enabled   = local.foundation_enabled && var.enable_origin_owner_acl_diagnostic_job
  observer_enabled               = local.foundation_enabled && var.enable_origin_observer_job
  import_infrastructure_enabled  = local.foundation_enabled && var.enable_origin_import_infrastructure
  import_enabled                 = local.foundation_enabled && var.enable_origin_import_job
  uploader_enabled               = local.foundation_enabled && var.enable_origin_ciphertext_uploader
  uploader_impersonation_enabled = local.foundation_enabled && var.enable_uploader_impersonation_binding
  observer_account_id            = "g03o-observer-${var.run_id}"
  postrestore_enabled            = local.foundation_enabled && var.enable_origin_postrestore_job
  recovery_enabled               = local.foundation_enabled && (var.enable_origin_recovery_plan_job || var.enable_origin_recovery_apply_job)
  recovery_apply_enabled         = local.foundation_enabled && var.enable_origin_recovery_apply_job
  postrestore_account_id         = "g03o-postverify-${var.run_id}"
  observer_database_url          = "postgres://${local.observer_account_id}%40${var.project_id}.iam@localhost:5432/espacigo_scratch?sslmode=disable"
}
