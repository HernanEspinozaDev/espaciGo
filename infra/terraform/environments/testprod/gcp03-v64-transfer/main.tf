locals {
  enabled        = var.enable_job
  execute        = local.enabled && var.transfer_mode == "execute"
  prefix         = "g03o-1017"
  source_db      = "espacigo_scratch"
  target_db      = "espacigo"
  source_files   = "espacigo-gcp03-private-g03o-1017"
  source_bucket  = "espacigo-gcp03-origin-g03o-1017"
  target_bucket  = "espacigo-test-files-447373903153-gcp02"
  source_conn    = "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg"
  target_conn    = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
  target_confirm = "${local.target_conn}/${local.target_db}:full-copy:g03o-1017"
  source_confirm = "${local.source_conn}/${local.source_db}"
}

data "google_sql_database_instance" "source" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  name    = "espacigo-gcp03-origin-g03o-1017-pg"
}

data "google_sql_database_instance" "target" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  name    = "espacigo-testprod-pg"
}

data "google_compute_network" "existing" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  name    = "espacigo-testprod"
}

data "google_compute_subnetwork" "existing" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  region  = var.region
  name    = "espacigo-testprod-serverless"
}

data "google_storage_bucket" "source_artifacts" {
  count = local.enabled ? 1 : 0
  name  = local.source_bucket
}

data "google_storage_bucket" "source_files" {
  count = local.enabled ? 1 : 0
  name  = local.source_files
}

data "google_storage_bucket" "target_files" {
  count = local.enabled ? 1 : 0
  name  = local.target_bucket
}

data "google_secret_manager_secret" "source_admin" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-gcp03-origin-g03o-1017-postgres-password"
}

data "google_secret_manager_secret" "source_transport" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-gcp03-origin-g03o-1017-transport-key"
}

data "google_secret_manager_secret" "target_migrator" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-test-database-migrator-password"
}

data "google_secret_manager_secret" "target_contract" {
  count     = local.enabled ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-test-contract-encryption-key"
}

data "google_cloud_run_v2_service" "api" {
  count    = local.enabled ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = "espacigo-test-api"
}

resource "google_service_account" "transfer" {
  count        = local.enabled ? 1 : 0
  project      = var.project_id
  account_id   = "g03o-v64-transfer"
  display_name = "G03O to primary V64 one-shot transfer"
  description  = "Run-scoped one-shot copy from G03O scratch into the empty primary V64 app database."
}

resource "google_project_iam_member" "source_sql_client" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-source-connector"
    description = "Connector access to the exact retained G03O origin instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/espacigo-gcp03-origin-g03o-1017-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_project_iam_member" "target_sql_client" {
  count   = local.enabled ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-target-connector"
    description = "Connector access to the exact existing primary instance."
    expression  = "resource.name == 'projects/${var.project_id}/instances/espacigo-testprod-pg' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

resource "google_secret_manager_secret_iam_member" "source_admin_reader" {
  count     = local.enabled && var.confirm_source_secrets ? 1 : 0
  project   = var.project_id
  secret_id = data.google_secret_manager_secret.source_admin[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.transfer[0].email}"
}

resource "google_secret_manager_secret_iam_member" "source_transport_reader" {
  count     = local.enabled && var.confirm_source_secrets ? 1 : 0
  project   = var.project_id
  secret_id = data.google_secret_manager_secret.source_transport[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.transfer[0].email}"
}

resource "google_secret_manager_secret_iam_member" "target_migrator_reader" {
  count     = local.enabled && var.confirm_target_migrator_secret ? 1 : 0
  project   = var.project_id
  secret_id = data.google_secret_manager_secret.target_migrator[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.transfer[0].email}"
}

resource "google_project_iam_custom_role" "contract_version_adder" {
  count       = local.execute ? 1 : 0
  project     = var.project_id
  role_id     = "g03oV64ContractVersionAdder"
  title       = "G03O V64 contract key version adder"
  description = "Append a version to the existing API contract encryption secret for the reviewed G03O transfer."
  permissions = ["secretmanager.versions.add"]
}

resource "google_secret_manager_secret_iam_member" "contract_version_adder" {
  count     = local.execute ? 1 : 0
  project   = var.project_id
  secret_id = data.google_secret_manager_secret.target_contract[0].secret_id
  role      = google_project_iam_custom_role.contract_version_adder[0].name
  member    = "serviceAccount:${google_service_account.transfer[0].email}"
}

resource "google_project_iam_custom_role" "source_object_reader" {
  count       = local.enabled ? 1 : 0
  project     = var.project_id
  role_id     = "g03oV64SourceObjectReader"
  title       = "G03O V64 exact source object reader"
  description = "Read exact artifact and private-file objects from the retained G03O run."
  permissions = ["storage.objects.get"]
}

resource "google_storage_bucket_iam_member" "source_artifact_reader" {
  count  = local.enabled ? 1 : 0
  bucket = data.google_storage_bucket.source_artifacts[0].name
  role   = google_project_iam_custom_role.source_object_reader[0].name
  member = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-origin-artifacts"
    description = "Read the existing authenticated G03O receipt, manifest and encrypted contract-key artifact."
    expression  = "resource.name.startsWith('projects/_/buckets/${local.source_bucket}/objects/runs/g03o-1017/')"
  }
}

resource "google_storage_bucket_iam_member" "source_file_reader" {
  count  = local.enabled ? 1 : 0
  bucket = data.google_storage_bucket.source_files[0].name
  role   = google_project_iam_custom_role.source_object_reader[0].name
  member = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-source-file-objects"
    description = "Read only the retained G03O private-file objects for this run."
    expression  = "resource.name.startsWith('projects/_/buckets/${local.source_files}/objects/runs/g03o-1017/')"
  }
}

resource "google_project_iam_custom_role" "target_private_objects" {
  count       = local.enabled ? 1 : 0
  project     = var.project_id
  role_id     = "g03oV64PrivatePrefixObjects"
  title       = "G03O V64 private prefix object transfer"
  description = "Read exact G03O V64 destination object names during preflight."
  permissions = ["storage.objects.get"]
}

resource "google_storage_bucket_iam_member" "target_private_objects" {
  count  = local.enabled && var.confirm_target_files_bucket ? 1 : 0
  bucket = data.google_storage_bucket.target_files[0].name
  role   = google_project_iam_custom_role.target_private_objects[0].name
  member = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-target-prefix"
    description = "Limit destination preflight reads to the dedicated G03O namespace."
    expression  = "resource.name.startsWith('projects/_/buckets/${local.target_bucket}/objects/${local.prefix}/')"
  }
}

resource "google_project_iam_custom_role" "target_private_objects_mutator" {
  count       = local.execute ? 1 : 0
  project     = var.project_id
  role_id     = "g03oV64PrivatePrefixMutator"
  title       = "G03O V64 private prefix object copy and rollback"
  description = "Create and delete only G03O V64 prefixed app objects during the separately gated transfer."
  permissions = ["storage.objects.create", "storage.objects.delete"]
}

resource "google_storage_bucket_iam_member" "target_private_objects_mutator" {
  count  = local.execute && var.confirm_target_files_bucket ? 1 : 0
  bucket = data.google_storage_bucket.target_files[0].name
  role   = google_project_iam_custom_role.target_private_objects_mutator[0].name
  member = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-target-prefix-mutator"
    description = "Limit create-only copies and generation-bound rollback deletes to the dedicated G03O namespace."
    expression  = "resource.name.startsWith('projects/_/buckets/${local.target_bucket}/objects/${local.prefix}/')"
  }
}

resource "google_project_iam_custom_role" "api_pause" {
  count       = local.execute ? 1 : 0
  project     = var.project_id
  role_id     = "g03oV64PauseApiService"
  title       = "G03O V64 transfer API pause"
  description = "Read and update only the existing API service to pause and safely resume transfer traffic."
  permissions = ["run.services.get", "run.services.update"]
}

resource "google_project_iam_member" "api_pause" {
  count   = local.execute ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.api_pause[0].name
  member  = "serviceAccount:${google_service_account.transfer[0].email}"
  condition {
    title       = "g03o-v64-exact-api-service"
    description = "Pause/update only espacigo-test-api in southamerica-west1."
    expression  = "resource.name == 'projects/${var.project_id}/locations/${var.region}/services/espacigo-test-api' && resource.type == 'run.googleapis.com/Service'"
  }
}

resource "google_cloud_run_v2_job" "transfer" {
  count               = local.enabled ? 1 : 0
  project             = var.project_id
  name                = "espacigo-g03o-v64-transfer"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.transfer[0].email
      timeout         = "14400s"
      max_retries     = 0

      containers {
        name       = "transfer"
        image      = var.transfer_image
        depends_on = ["cloud-sql-proxy-source", "cloud-sql-proxy-target"]
        env {
          name  = "GCP03_V64_TRANSFER_CONFIRM_TARGET"
          value = local.target_confirm
        }
        env {
          name  = "GCP03_V64_TRANSFER_EXECUTE"
          value = local.execute ? "YES" : "NO"
        }
        env {
          name  = "GCP03_V64_TRANSFER_REVIEWED_RELEASE_SHA256"
          value = local.execute ? var.reviewed_release_sha256 : ""
        }
        env {
          name  = "GCP03_V64_SOURCE_PASSWORD_FILE"
          value = "/secrets/source-admin/password"
        }
        env {
          name  = "GCP03_V64_TARGET_PASSWORD_FILE"
          value = "/secrets/target-migrator/password"
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_BUCKET"
          value = local.source_bucket
        }
        env {
          name  = "GCP03_ORIGIN_PRIVATE_FILES_BUCKET"
          value = local.source_files
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT"
          value = "runs/g03o-1017/receipt.json"
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION"
          value = var.receipt_generation
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_SHA256"
          value = var.receipt_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_RECEIPT_BYTES"
          value = tostring(var.receipt_bytes)
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_MANIFEST_SHA256"
          value = var.manifest_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_BOOTSTRAP_SHA256"
          value = var.bootstrap_report_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_OBSERVER_SHA256"
          value = var.observer_report_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_NORMALIZATION_SHA256"
          value = var.normalization_sql_sha256
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_CONFIRM_TARGET"
          value = local.source_confirm
        }
        env {
          name  = "GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE"
          value = "/secrets/source-transport/k_run"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = local.source_conn
        }
        env {
          name  = "CLOUD_SQL_PRIVATE_IP"
          value = "true"
        }
        env {
          name  = "DATABASE_CONNECTION_MODE"
          value = "cloudsql"
        }
        env {
          name  = "DATABASE_URL"
          value = "postgres://espacigo_migrator_login@127.0.0.1:5433/espacigo?sslmode=disable"
        }

        volume_mounts {
          name       = "source-admin"
          mount_path = "/secrets/source-admin"
        }
        volume_mounts {
          name       = "target-migrator"
          mount_path = "/secrets/target-migrator"
        }
        volume_mounts {
          name       = "source-transport"
          mount_path = "/secrets/source-transport"
        }
        resources {
          limits = {
            cpu    = "2"
            memory = "4Gi"
          }
        }
      }

      containers {
        name    = "cloud-sql-proxy-source"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--address=0.0.0.0", "--port=5432", local.source_conn]
        startup_probe {
          tcp_socket {
            port = 5432
          }
          period_seconds    = 1
          timeout_seconds   = 1
          failure_threshold = 90
        }
        resources {
          limits = {
            cpu    = "1"
            memory = "256Mi"
          }
        }
      }

      containers {
        name    = "cloud-sql-proxy-target"
        image   = var.proxy_image
        command = ["/cloud-sql-proxy"]
        args    = ["--private-ip", "--address=0.0.0.0", "--port=5433", local.target_conn]
        startup_probe {
          tcp_socket {
            port = 5433
          }
          period_seconds    = 1
          timeout_seconds   = 1
          failure_threshold = 90
        }
        resources {
          limits = {
            cpu    = "1"
            memory = "256Mi"
          }
        }
      }

      volumes {
        name = "source-admin"
        secret {
          secret       = data.google_secret_manager_secret.source_admin[0].secret_id
          default_mode = 292
          items {
            version = var.source_admin_secret_version
            path    = "password"
            mode    = 292
          }
        }
      }
      volumes {
        name = "target-migrator"
        secret {
          secret       = data.google_secret_manager_secret.target_migrator[0].secret_id
          default_mode = 292
          items {
            version = var.target_migrator_secret_version
            path    = "password"
            mode    = 292
          }
        }
      }
      volumes {
        name = "source-transport"
        secret {
          secret       = data.google_secret_manager_secret.source_transport[0].secret_id
          default_mode = 292
          items {
            version = var.source_transport_secret_version
            path    = "k_run"
            mode    = 292
          }
        }
      }
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = data.google_compute_network.existing[0].name
          subnetwork = data.google_compute_subnetwork.existing[0].name
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (local.enabled && data.google_sql_database_instance.source[0].connection_name == local.source_conn &&
        data.google_sql_database_instance.target[0].connection_name == local.target_conn &&
        data.google_cloud_run_v2_service.api[0].name == "espacigo-test-api" &&
      data.google_compute_subnetwork.existing[0].region == var.region)
      error_message = "The transfer Job is pinned to the existing G03O source, primary V64 target, API service and private subnet."
    }
    precondition {
      condition = (!local.enabled || (var.confirm_source_secrets && var.confirm_target_migrator_secret && var.confirm_target_files_bucket &&
        can(regex("^[1-9][0-9]*$", var.source_admin_secret_version)) &&
        can(regex("^[1-9][0-9]*$", var.source_transport_secret_version)) &&
        can(regex("^[1-9][0-9]*$", var.target_migrator_secret_version)) &&
        can(regex("^[0-9a-f]{64}$", var.receipt_sha256)) && can(regex("^[0-9a-f]{64}$", var.manifest_sha256)) &&
        can(regex("^[0-9a-f]{64}$", var.bootstrap_report_sha256)) && can(regex("^[0-9a-f]{64}$", var.observer_report_sha256)) &&
        can(regex("^[0-9a-f]{64}$", var.normalization_sql_sha256)) && can(regex("^[1-9][0-9]*$", var.receipt_generation)) &&
      var.receipt_bytes > 0 && can(regex("@sha256:[0-9a-f]{64}$", var.transfer_image)) && can(regex("@sha256:[0-9a-f]{64}$", var.proxy_image))))
      error_message = "The Job requires reviewed numeric secret versions, exact artifact evidence and digest-pinned images."
    }
    precondition {
      condition     = !local.execute || (var.confirm_transfer_execution && can(regex("^[0-9a-f]{64}$", var.reviewed_release_sha256)))
      error_message = "Execute mode requires an explicit separate execution acknowledgment and reviewed release fingerprint."
    }
  }
}
