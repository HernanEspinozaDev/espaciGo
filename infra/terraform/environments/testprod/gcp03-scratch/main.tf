locals {
  name_prefix                                = "espacigo-gcp03-${var.run_id}"
  scratch_enabled                            = var.enable_scratch
  job_enabled                                = var.enable_scratch && var.enable_job
  observer_job_enabled                       = var.enable_scratch && var.enable_observer_job
  owner_history_job_enabled                  = var.enable_scratch && var.enable_owner_history_grant_job
  retired_verification_job_enabled           = var.enable_scratch && var.enable_retired_verification_job
  retired_membership_revoke_job_enabled      = var.enable_scratch && var.enable_retired_membership_revoke_job
  role_probe_enabled                         = var.enable_scratch && var.enable_role_probe_jobs
  role_probe_credential_permissions_enabled  = var.enable_scratch && var.enable_role_probe_credential_permissions
  replay_credential_permissions_enabled      = var.enable_scratch && var.enable_replay_credential_permissions
  scratch_migrator_service_account_email     = "gcp03-scratch-${var.run_id}@${var.project_id}.iam.gserviceaccount.com"
  scratch_database_acl_service_account_email = "gcp03-scratch-acl-${var.run_id}@${var.project_id}.iam.gserviceaccount.com"
  scratch_migrator_service_account_id        = "projects/${var.project_id}/serviceAccounts/${local.scratch_migrator_service_account_email}"
  scratch_database_acl_service_account_id    = "projects/${var.project_id}/serviceAccounts/${local.scratch_database_acl_service_account_email}"
  protect_scratch                            = !var.allow_scratch_destroy
  protect_replay_jobs                        = !var.allow_replay_job_destroy && !var.allow_gcp03_job_deletion_protection_disable
  protect_gcp03_phase_jobs                   = !var.allow_gcp03_job_deletion_protection_disable
  protect_role_probe_jobs                    = !var.allow_role_probe_job_destroy
  job_phases = {
    bootstrap    = "bootstrap"
    database_acl = "database_acl"
    migrate      = "migrate"
    finalize     = "finalize"
    retire       = "retire"
  }
  role_probe_phases = {
    role_comment_prepare         = "role_comment_prepare"
    role_comment_after_reconcile = "role_comment_after_reconcile"
  }
}

data "google_compute_network" "existing" {
  name    = var.network_name
  project = var.project_id
}

data "google_compute_subnetwork" "existing" {
  name    = var.subnetwork_name
  region  = var.region
  project = var.project_id
}

data "google_project" "current" {
  project_id = var.project_id
}

resource "google_service_account" "scratch_migrator" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  account_id      = "gcp03-scratch-${var.run_id}"
  display_name    = "GCP-03 disposable migration runner ${var.run_id}"
  description     = "Scratch only. No access to persistent database or application secrets."
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

resource "google_service_account" "scratch_database_acl" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  account_id      = "gcp03-scratch-acl-${var.run_id}"
  display_name    = "GCP-03 scratch database ACL phase ${var.run_id}"
  description     = "Scratch ACL phase only. Reads only the short-lived database-owner secret."
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

resource "google_project_iam_member" "scratch_cloudsql_client" {
  count   = local.scratch_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.scratch_migrator[0].email}"

  condition {
    title       = "gcp03-${var.run_id}-instance"
    description = "Limit the temporary migrator to its disposable Cloud SQL instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.name_prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_project_iam_member" "scratch_acl_cloudsql_client" {
  count   = local.scratch_enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.scratch_database_acl[0].email}"

  condition {
    title       = "gcp03-${var.run_id}-instance"
    description = "Limit the temporary database-ACL identity to its disposable Cloud SQL instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/${local.name_prefix}-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_secret_manager_secret" "scratch_password" {
  count               = local.scratch_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = "${local.name_prefix}-migrator-password"
  deletion_protection = local.protect_scratch
  deletion_policy     = local.protect_scratch ? "PREVENT" : "DELETE"

  replication {
    auto {}
  }

  # Terraform creates metadata only; no secret version or value is managed here.
}

resource "google_secret_manager_secret" "scratch_database_owner_password" {
  count               = local.scratch_enabled ? 1 : 0
  project             = var.project_id
  secret_id           = "${local.name_prefix}-db-owner-password"
  deletion_protection = local.protect_scratch
  deletion_policy     = local.protect_scratch ? "PREVENT" : "DELETE"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "scratch_password_access" {
  count     = local.scratch_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.scratch_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.scratch_migrator[0].email}"
}

resource "google_secret_manager_secret_iam_member" "scratch_database_owner_access" {
  count     = local.scratch_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.scratch_database_owner_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.scratch_database_acl[0].email}"
}

resource "google_artifact_registry_repository" "scratch" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  location        = var.region
  repository_id   = "gcp03-scratch-${var.run_id}"
  description     = "Digest-pinned disposable GCP-03 migrator image ${var.run_id}."
  format          = "DOCKER"
  mode            = "STANDARD_REPOSITORY"
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

# Cloud Run's service agent pulls the image; the runtime identity receives no registry role.
resource "google_artifact_registry_repository_iam_member" "cloud_run_reader" {
  count      = local.scratch_enabled ? 1 : 0
  project    = var.project_id
  location   = google_artifact_registry_repository.scratch[0].location
  repository = google_artifact_registry_repository.scratch[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:service-${data.google_project.current.number}@serverless-robot-prod.iam.gserviceaccount.com"
}

resource "google_sql_database_instance" "scratch" {
  count               = local.scratch_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.name_prefix}-pg"
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = local.protect_scratch
  deletion_policy     = local.protect_scratch ? "PREVENT" : "DELETE"

  settings {
    edition                     = "ENTERPRISE"
    tier                        = var.instance_tier
    availability_type           = "ZONAL"
    connector_enforcement       = "REQUIRED"
    deletion_protection_enabled = local.protect_scratch
    disk_type                   = "HYPERDISK_BALANCED"
    disk_size                   = 20
    disk_autoresize             = false
    user_labels = {
      application = "espacigo"
      environment = "scratch"
      purpose     = "gcp03-compatibility"
      run_id      = replace(var.run_id, "-", "_")
      managed_by  = "terraform"
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = data.google_compute_network.existing.self_link
      ssl_mode        = "ENCRYPTED_ONLY"
    }

    backup_configuration {
      enabled                        = false
      point_in_time_recovery_enabled = false
    }
  }

  lifecycle {
    prevent_destroy = false
    precondition {
      condition     = data.google_compute_subnetwork.existing.private_ip_google_access
      error_message = "Existing subnet must have Private Google Access enabled before a future scratch plan/apply."
    }
    precondition {
      condition     = var.region == data.google_compute_subnetwork.existing.region
      error_message = "Scratch Cloud SQL and Direct VPC subnet must use the same region."
    }
  }
}

resource "google_sql_database" "scratch" {
  count           = local.scratch_enabled ? 1 : 0
  project         = var.project_id
  name            = var.database_name
  instance        = google_sql_database_instance.scratch[0].name
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = local.protect_scratch ? "PREVENT" : "DELETE"
}

resource "google_cloud_run_v2_job" "migrator" {
  for_each            = local.job_enabled ? local.job_phases : {}
  project             = var.project_id
  name                = "${local.name_prefix}-${replace(each.key, "_", "-")}"
  location            = var.region
  deletion_protection = local.protect_replay_jobs

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = contains(["database_acl", "retire"], each.key) ? local.scratch_database_acl_service_account_email : local.scratch_migrator_service_account_email
      timeout         = "14400s"
      max_retries     = 0

      containers {
        image = var.migrator_image
        env {
          name  = "GCP03_PHASE"
          value = each.value
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = contains(["database_acl", "retire"], each.key) ? "postgres://postgres@localhost:5432/${var.database_name}?sslmode=disable" : "postgres://${var.database_user}@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = contains(["database_acl", "retire"], each.key) ? "/secrets/database-owner/password" : "/secrets/migrator/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        env {
          name  = "DBMIGRATE_TIMEOUT"
          value = "4h"
        }
        volume_mounts {
          name       = contains(["database_acl", "retire"], each.key) ? "scratch-database-owner-password" : "scratch-migrator-password"
          mount_path = contains(["database_acl", "retire"], each.key) ? "/secrets/database-owner" : "/secrets/migrator"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = contains(["database_acl", "retire"], each.key) ? "scratch-database-owner-password" : "scratch-migrator-password"
        secret {
          secret = contains(["database_acl", "retire"], each.key) ? google_secret_manager_secret.scratch_database_owner_password[0].secret_id : google_secret_manager_secret.scratch_password[0].secret_id
          items {
            version = contains(["database_acl", "retire"], each.key) ? var.database_owner_secret_version : var.migrator_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.scratch_enabled && var.confirm_image_pushed &&
        var.confirm_scratch_secret_version && var.confirm_database_owner_secret_version &&
        can(regex("^[1-9][0-9]*$", var.migrator_secret_version)) &&
        can(regex("^[1-9][0-9]*$", var.database_owner_secret_version))
      )
      error_message = "The named phases require the reviewed image digest and exact active numeric migrator/database-owner secret versions."
    }
  }
}

resource "google_cloud_run_v2_job" "observer" {
  count               = local.observer_job_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.name_prefix}-observe"
  location            = var.region
  deletion_protection = local.protect_gcp03_phase_jobs

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = local.scratch_migrator_service_account_email
      timeout         = "1800s"
      max_retries     = 0

      containers {
        image = var.observer_image
        env {
          name  = "GCP03_PHASE"
          value = "observe"
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://${var.database_user}@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/migrator/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        env {
          name  = "DBMIGRATE_TIMEOUT"
          value = "4h"
        }
        volume_mounts {
          name       = "scratch-migrator-password"
          mount_path = "/secrets/migrator"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = "scratch-migrator-password"
        secret {
          secret = google_secret_manager_secret.scratch_password[0].secret_id
          items {
            version = var.migrator_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.observer_job_enabled && local.job_enabled && var.confirm_observer_image_pushed &&
        var.confirm_scratch_secret_version && can(regex("^[1-9][0-9]*$", var.migrator_secret_version)) &&
        var.operator_member == "user:pasteleriahijitos@gmail.com"
      )
      error_message = "The observer Job requires its separate gate, the existing fixed five-job target, a verified digest image, the exact operator, and an active numeric migrator secret version."
    }
  }
}

resource "google_cloud_run_v2_job" "owner_history_grant" {
  count               = local.owner_history_job_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.name_prefix}-grant-owner-history"
  location            = var.region
  deletion_protection = local.protect_gcp03_phase_jobs

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = local.scratch_migrator_service_account_email
      timeout         = "1800s"
      max_retries     = 0

      containers {
        image = var.owner_history_image
        env {
          name  = "GCP03_PHASE"
          value = "grant_owner_history"
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://${var.database_user}@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/migrator/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        volume_mounts {
          name       = "scratch-migrator-password"
          mount_path = "/secrets/migrator"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = "scratch-migrator-password"
        secret {
          secret = google_secret_manager_secret.scratch_password[0].secret_id
          items {
            version = var.migrator_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.owner_history_job_enabled && local.job_enabled && var.confirm_owner_history_image_pushed &&
        var.confirm_scratch_secret_version && can(regex("^[1-9][0-9]*$", var.migrator_secret_version)) &&
        var.operator_member == "user:pasteleriahijitos@gmail.com"
      )
      error_message = "The owner-history Job requires the independent gate, existing fixed migration Jobs, reviewed digest, exact operator, and active numeric migrator secret version."
    }
  }
}

resource "google_cloud_run_v2_job" "retired_verification" {
  count               = local.retired_verification_job_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.name_prefix}-verify-retired"
  location            = var.region
  deletion_protection = local.protect_gcp03_phase_jobs

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = local.scratch_database_acl_service_account_email
      timeout         = "1800s"
      max_retries     = 0

      containers {
        image = var.retired_verification_image
        env {
          name  = "GCP03_PHASE"
          value = "verify_retired"
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://postgres@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/database-owner/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        volume_mounts {
          name       = "scratch-database-owner-password"
          mount_path = "/secrets/database-owner"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = "scratch-database-owner-password"
        secret {
          secret = google_secret_manager_secret.scratch_database_owner_password[0].secret_id
          items {
            version = var.database_owner_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.retired_verification_job_enabled && var.confirm_retired_verification_image_pushed &&
        var.confirm_database_owner_secret_version && can(regex("^[1-9][0-9]*$", var.database_owner_secret_version)) &&
        var.operator_member == "user:pasteleriahijitos@gmail.com" && !var.allow_scratch_destroy &&
        !var.allow_replay_job_destroy
      )
      error_message = "The read-only retired verification Job requires its independent gate, exact owner secret, reviewed image, exact operator, and protected scratch target."
    }
  }
}

resource "google_cloud_run_v2_job" "retired_membership_revoke" {
  count               = local.retired_membership_revoke_job_enabled ? 1 : 0
  project             = var.project_id
  name                = "${local.name_prefix}-revoke-retired-membership"
  location            = var.region
  deletion_protection = local.protect_gcp03_phase_jobs

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = local.scratch_database_acl_service_account_email
      timeout         = "1800s"
      max_retries     = 0

      containers {
        image = var.retired_membership_revoke_image
        env {
          name  = "GCP03_PHASE"
          value = "revoke_retired_membership"
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://postgres@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/database-owner/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        volume_mounts {
          name       = "scratch-database-owner-password"
          mount_path = "/secrets/database-owner"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      volumes {
        name = "scratch-database-owner-password"
        secret {
          secret = google_secret_manager_secret.scratch_database_owner_password[0].secret_id
          items {
            version = var.database_owner_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.retired_membership_revoke_job_enabled && var.confirm_retired_membership_revoke_image_pushed &&
        var.confirm_database_owner_secret_version && can(regex("^[1-9][0-9]*$", var.database_owner_secret_version)) &&
        var.operator_member == "user:pasteleriahijitos@gmail.com" && !var.allow_scratch_destroy &&
        !var.allow_replay_job_destroy
      )
      error_message = "The retired-membership revocation Job requires its independent gate, exact owner secret, reviewed image, exact operator, and protected scratch target."
    }
  }
}

resource "google_cloud_run_v2_job" "role_comment_probe" {
  for_each            = local.role_probe_enabled ? local.role_probe_phases : {}
  project             = var.project_id
  name                = "${local.name_prefix}-${replace(each.key, "_", "-")}"
  location            = var.region
  deletion_protection = local.protect_role_probe_jobs

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = google_service_account.scratch_migrator[0].email
      timeout         = "600s"
      max_retries     = 0

      containers {
        image = var.role_probe_image
        env {
          name  = "GCP03_PHASE"
          value = each.value
        }
        env {
          name  = "GCP03_CONFIRM_TARGET"
          value = "${google_sql_database_instance.scratch[0].connection_name}/${google_sql_database.scratch[0].name}"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://${var.database_user}@localhost:5432/${var.database_name}?sslmode=disable"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/secrets/migrator/password"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.scratch[0].connection_name
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        volume_mounts {
          name       = "gcp03-role-comment-probe-password"
          mount_path = "/secrets/migrator"
        }
        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }
      }

      volumes {
        name = "gcp03-role-comment-probe-password"
        secret {
          secret = google_secret_manager_secret.scratch_password[0].secret_id
          items {
            version = var.role_probe_secret_version
            path    = "password"
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing.name
          subnetwork = data.google_compute_subnetwork.existing.name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.role_probe_enabled && var.confirm_role_probe_image_pushed &&
        var.confirm_role_probe_secret_version && var.role_probe_secret_version != "latest"
      )
      error_message = "The fixed role-comment probe requires a digest-pinned uploaded image and an exact numeric scratch secret version."
    }
  }
}

resource "google_project_iam_custom_role" "role_probe_cloudsql_operator" {
  count       = local.role_probe_credential_permissions_enabled ? 1 : 0
  project     = var.project_id
  role_id     = "gcp03roleprobe_sqlops"
  title       = "GCP-03 temporary role probe SQL user operations"
  description = "Temporary exact Cloud SQL Admin API methods for the fixed GCP-03 role-comment probe."
  permissions = [
    "cloudsql.instances.get",
    "cloudsql.users.create",
    "cloudsql.users.delete",
    "cloudsql.users.get",
    "cloudsql.users.update",
  ]
}

resource "google_project_iam_member" "role_probe_cloudsql_operator" {
  count   = local.role_probe_credential_permissions_enabled ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.role_probe_cloudsql_operator[0].name
  member  = var.operator_member

  condition {
    title       = "gcp03-${var.run_id}-role-probe-expiry"
    description = "Short-lived project-scoped Cloud SQL user operations for the fixed GCP-03 probe; applies to Cloud SQL instances in this project."
    expression  = "request.time < timestamp('${var.role_probe_credential_iam_expires_at}')"
  }
}

resource "google_project_iam_custom_role" "role_probe_secret_operator" {
  count       = local.role_probe_credential_permissions_enabled ? 1 : 0
  project     = var.project_id
  role_id     = "gcp03roleprobe_secretops"
  title       = "GCP-03 temporary role probe secret version operations"
  description = "Temporary version management for the single scratch migrator secret."
  permissions = [
    "secretmanager.versions.add",
    "secretmanager.versions.destroy",
    "secretmanager.versions.get",
    "secretmanager.versions.list",
  ]
}

resource "google_secret_manager_secret_iam_member" "role_probe_secret_operator" {
  count     = local.role_probe_credential_permissions_enabled ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.scratch_password[0].secret_id
  role      = google_project_iam_custom_role.role_probe_secret_operator[0].name
  member    = var.operator_member

  condition {
    title       = "gcp03-${var.run_id}-credential-permissions-expiry"
    description = "Short-lived version operations on the exact scratch migrator secret."
    expression  = "request.time < timestamp('${var.role_probe_credential_iam_expires_at}')"
  }
}

# Separate full-replay credential gate. These custom role IDs are distinct from
# the soft-deleted role-probe IDs and are never granted to runtime service accounts.
resource "google_project_iam_custom_role" "replay_credential_sql_operator" {
  count       = local.replay_credential_permissions_enabled ? 1 : 0
  project     = var.project_id
  role_id     = "gcp03v1v61sqlops261016"
  title       = "GCP-03 V1-V61 temporary SQL credential operator"
  description = "Short-lived Cloud SQL Admin API methods for the fixed scratch migrator and postgres-owner logins."
  permissions = [
    "cloudsql.instances.get",
    "cloudsql.users.create",
    "cloudsql.users.get",
    "cloudsql.users.update",
  ]
}

resource "google_project_iam_member" "replay_credential_sql_operator" {
  count   = local.replay_credential_permissions_enabled ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.replay_credential_sql_operator[0].name
  member  = var.operator_member

  condition {
    title       = "gcp03-${var.run_id}-replay-sql-credential-expiry"
    description = "Short-lived project-scoped SQL user operations for only the named scratch procedure; Cloud SQL Admin IAM cannot constrain users.* to one instance."
    expression  = "request.time < timestamp('${var.replay_credential_iam_expires_at}')"
  }
}

resource "google_project_iam_custom_role" "replay_secret_version_operator" {
  count       = local.replay_credential_permissions_enabled ? 1 : 0
  project     = var.project_id
  role_id     = "gcp03v1v61secretversions261016"
  title       = "GCP-03 V1-V61 temporary secret version operator"
  description = "Short-lived version metadata/add/destroy operations on the two fixed scratch credential secrets; excludes payload access."
  permissions = [
    "secretmanager.versions.add",
    "secretmanager.versions.destroy",
    "secretmanager.versions.get",
    "secretmanager.versions.list",
  ]
}

resource "google_secret_manager_secret_iam_member" "replay_migrator_secret_version_operator" {
  count     = local.replay_credential_permissions_enabled ? 1 : 0
  project   = var.project_id
  secret_id = "projects/${var.project_id}/secrets/${google_secret_manager_secret.scratch_password[0].secret_id}"
  role      = google_project_iam_custom_role.replay_secret_version_operator[0].name
  member    = var.operator_member

  condition {
    title       = "gcp03-${var.run_id}-replay-migrator-secret-expiry"
    description = "Short-lived version operations on the exact scratch migrator credential secret; no secret payload access."
    expression  = "request.time < timestamp('${var.replay_credential_iam_expires_at}')"
  }
}

resource "google_secret_manager_secret_iam_member" "replay_owner_secret_version_operator" {
  count     = local.replay_credential_permissions_enabled ? 1 : 0
  project   = var.project_id
  secret_id = "projects/${var.project_id}/secrets/${google_secret_manager_secret.scratch_database_owner_password[0].secret_id}"
  role      = google_project_iam_custom_role.replay_secret_version_operator[0].name
  member    = var.operator_member

  condition {
    title       = "gcp03-${var.run_id}-replay-owner-secret-expiry"
    description = "Short-lived version operations on the exact scratch database-owner credential secret; no secret payload access."
    expression  = "request.time < timestamp('${var.replay_credential_iam_expires_at}')"
  }
}

resource "google_cloud_run_v2_job_iam_member" "invoke_operator" {
  for_each = google_cloud_run_v2_job.migrator
  project  = var.project_id
  location = each.value.location
  name     = each.value.name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_cloud_run_v2_job_iam_member" "observer_operator" {
  count    = local.observer_job_enabled ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_job.observer[0].location
  name     = google_cloud_run_v2_job.observer[0].name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_cloud_run_v2_job_iam_member" "owner_history_operator" {
  count    = local.owner_history_job_enabled ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_job.owner_history_grant[0].location
  name     = google_cloud_run_v2_job.owner_history_grant[0].name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_cloud_run_v2_job_iam_member" "retired_verification_operator" {
  count    = local.retired_verification_job_enabled ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_job.retired_verification[0].location
  name     = google_cloud_run_v2_job.retired_verification[0].name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_cloud_run_v2_job_iam_member" "retired_membership_revoke_operator" {
  count    = local.retired_membership_revoke_job_enabled ? 1 : 0
  project  = var.project_id
  location = google_cloud_run_v2_job.retired_membership_revoke[0].location
  name     = google_cloud_run_v2_job.retired_membership_revoke[0].name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_service_account_iam_member" "operator_act_as" {
  count              = local.job_enabled || local.role_probe_enabled ? 1 : 0
  service_account_id = local.scratch_migrator_service_account_id
  role               = "roles/iam.serviceAccountUser"
  member             = var.operator_member
}

resource "google_cloud_run_v2_job_iam_member" "role_probe_operator" {
  for_each = google_cloud_run_v2_job.role_comment_probe
  project  = var.project_id
  location = each.value.location
  name     = each.value.name
  role     = "roles/run.invoker"
  member   = var.operator_member
}

resource "google_service_account_iam_member" "operator_act_as_database_acl" {
  count              = local.job_enabled || local.retired_verification_job_enabled || local.retired_membership_revoke_job_enabled ? 1 : 0
  service_account_id = local.scratch_database_acl_service_account_id
  role               = "roles/iam.serviceAccountUser"
  member             = var.operator_member
}
