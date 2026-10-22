terraform {
  required_version = ">= 1.16.0, < 1.17.0"

  backend "gcs" {}

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 8.7.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 8.7.0"
    }
  }
}

provider "google" {
  project = var.project_id != "" ? var.project_id : null
  region  = var.region
}

provider "google-beta" {
  project = var.project_id != "" ? var.project_id : null
  region  = var.region
}

locals {
  api_gcp06_candidate_enabled = var.api_gcp06_candidate_run_id != ""
  gcp06_candidate_run_id      = local.api_gcp06_candidate_enabled ? var.api_gcp06_candidate_run_id : (var.enable_gcp06_jobs_reviewed && var.enable_gcp06_sql_inspector_job ? "20261020-gcp06a" : "")
  # V67 uses an externally provisioned password secret and must never recreate
  # the retired generic database-password secret. Candidate mode retains its
  # scoped secret and the prior generic secret only for the rollback revision.
  api_database_secret_key = local.api_gcp06_candidate_enabled ? "database-password-gcp06" : null
  api_secret_access_keys = setunion(
    toset([
      "contract-encryption-key",
      "payment-webhook-secret",
    ]),
    local.api_gcp06_candidate_enabled ? toset([local.api_database_secret_key]) : toset([]),
    # Keep the stable DB credential available to the retained rollback revision.
    local.api_gcp06_candidate_enabled ? toset(["database-password"]) : toset([]),
  )
  runtime_secret_ids = var.enable_foundation ? setunion(
    local.api_secret_access_keys,
    # Preserve the legacy API credential only for the GCP-06 candidate rollback.
    local.api_gcp06_candidate_enabled ? toset(["database-password"]) : toset([]),
    var.enable_database ? toset(["database-migrator-password"]) : toset([]),
    (var.enable_gcp06_jobs_reviewed && var.enable_gcp06_admin_bootstrap_job) ? toset(["gcp06-admin-bootstrap-password"]) : toset([]),
    (var.enable_gcp06_jobs_reviewed && var.enable_gcp06_test_access_job) ? toset(["gcp06-test-access-db-password"]) : toset([]),
    (var.enable_gcp06_jobs_reviewed && var.enable_gcp06_test_access_job && var.gcp06_test_access_mode == "apply") ? toset(["gcp06-test-account-password"]) : toset([]),
    (var.enable_gcp06_jobs_reviewed && var.enable_gcp06_sql_inspector_job) ? toset(["gcp06-sql-inspector-credential"]) : toset([]),
  ) : toset([])
  migrator_database_secret_id   = var.enable_database ? "database-migrator-password" : null
  gcp06_candidate_instance_name = "espacigo-gcp06-${local.gcp06_candidate_run_id}-pg"
  gcp06_candidate_database_name = "espacigo_gcp06_candidate_${local.gcp06_candidate_run_id}"
  project_enabled_apis = toset([
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "containeranalysis.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "iap.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "servicenetworking.googleapis.com",
    "sqladmin.googleapis.com",
    "sts.googleapis.com",
    "storage.googleapis.com",
    "serviceusage.googleapis.com",
  ])
  web_services = {
    public = {
      name         = var.public_service_name
      entry_mode   = "public"
      public_host  = "https://${var.public_hostname}"
      private_host = "https://${var.private_hostname}"
      iap_enabled  = false
    }
    private = {
      name         = var.private_service_name
      entry_mode   = "private"
      public_host  = "https://${var.private_hostname}"
      private_host = ""
      iap_enabled  = true
    }
  }
  mailpit_image           = "${var.region}-docker.pkg.dev/${var.project_id}/espacigo-testprod-dockerhub/axllent/mailpit@sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48"
  runtime_mailpit_address = var.enable_mailpit ? "${google_compute_address.mailpit[0].address}:1025" : var.mailpit_private_address
}

# Cloud Run app services and their service/IAP bindings move to Cloud Build.
# Forget the known Terraform bindings without deleting cloud resources.
removed {
  from = google_cloud_run_v2_service.api
  lifecycle { destroy = false }
}
removed {
  from = google_cloud_run_v2_service.web
  lifecycle { destroy = false }
}
removed {
  from = google_cloud_run_v2_service_iam_member.api_invoker
  lifecycle { destroy = false }
}
removed {
  from = google_cloud_run_v2_service_iam_member.deployer_service_updater
  lifecycle { destroy = false }
}
removed {
  from = google_cloud_run_v2_service_iam_member.public_web_invoker
  lifecycle { destroy = false }
}
removed {
  from = google_cloud_run_v2_service_iam_member.private_iap_invoker
  lifecycle { destroy = false }
}
removed {
  from = google_iap_web_cloud_run_service_iam_member.private_evaluators
  lifecycle { destroy = false }
}
removed {
  from = google_service_account_iam_member.deployer_act_as_runtime
  lifecycle { destroy = false }
}
removed {
  from = google_sql_database_instance.postgres
  lifecycle { destroy = false }
}
removed {
  from = google_sql_database.application
  lifecycle { destroy = false }
}
removed {
  from = google_project_service_identity.iap
  lifecycle { destroy = false }
}

data "google_sql_database_instance" "gcp06_candidate" {
  count   = local.api_gcp06_candidate_enabled || (var.enable_gcp06_jobs_reviewed && var.enable_gcp06_sql_inspector_job) ? 1 : 0
  project = var.project_id
  name    = local.gcp06_candidate_instance_name
}

resource "google_project_service" "required" {
  for_each                   = var.enable_foundation ? local.project_enabled_apis : toset([])
  project                    = var.project_id
  service                    = each.value
  disable_dependent_services = false
  disable_on_destroy         = false
}

resource "google_compute_network" "testprod" {
  count                   = var.enable_foundation ? 1 : 0
  name                    = "espacigo-testprod"
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"
  # Private Google Access uses the VPC's default internet-gateway route for eligible Google service IPs.
  delete_default_routes_on_create = false

  depends_on = [google_project_service.required]
}

resource "google_compute_subnetwork" "serverless" {
  count                    = var.enable_foundation ? 1 : 0
  name                     = "espacigo-testprod-serverless"
  region                   = var.region
  network                  = google_compute_network.testprod[0].id
  ip_cidr_range            = var.serverless_subnet_cidr
  private_ip_google_access = true
  purpose                  = "PRIVATE"
}

resource "google_compute_subnetwork" "api_serverless" {
  count                    = var.enable_foundation && var.enable_mailpit ? 1 : 0
  name                     = "espacigo-testprod-api-serverless"
  region                   = var.region
  network                  = google_compute_network.testprod[0].name
  ip_cidr_range            = var.api_serverless_subnet_cidr
  private_ip_google_access = true
  purpose                  = "PRIVATE"
}

resource "google_compute_subnetwork" "mailpit" {
  count                    = var.enable_foundation && var.enable_mailpit ? 1 : 0
  name                     = "espacigo-testprod-mailpit"
  region                   = var.region
  network                  = google_compute_network.testprod[0].name
  ip_cidr_range            = var.mailpit_subnet_cidr
  private_ip_google_access = true
  purpose                  = "PRIVATE"
}

resource "google_compute_global_address" "private_services" {
  count         = (var.enable_database || var.enable_private_services_access) ? 1 : 0
  name          = "espacigo-testprod-private-services"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  ip_version    = "IPV4"
  prefix_length = 16
  network       = google_compute_network.testprod[0].id
}

resource "google_service_networking_connection" "private_services" {
  count                   = (var.enable_database || var.enable_private_services_access) ? 1 : 0
  network                 = google_compute_network.testprod[0].id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_services[0].name]

  depends_on = [google_project_service.required]
}

resource "google_service_account" "runtime" {
  for_each     = var.enable_foundation ? toset(["api", "public-web", "private-web", "migrator", "deployer"]) : toset([])
  account_id   = "espacigo-test-${each.key}"
  display_name = "EspaciGo testprod ${each.key} runtime"

  depends_on = [google_project_service.required]
}

resource "google_service_account" "cloud_build_deployer" {
  count        = var.enable_foundation ? 1 : 0
  project      = var.project_id
  account_id   = "espacigo-cloud-build-deployer"
  display_name = "EspaciGo Cloud Build deployer"
  description  = "Dedicated identity for building/publishing images and deploying the three testprod Cloud Run services. No SQL or application secret access."

  depends_on = [google_project_service.required]
}

resource "google_project_iam_custom_role" "cloud_build_deployer" {
  count       = var.enable_foundation ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoCloudBuildDeployer"
  title       = "EspaciGo Cloud Build deployer"
  description = "Create/update/list/get the application Cloud Run services, their IAM and revisions, and submit/read the child image build."
  stage       = "GA"
  permissions = [
    "cloudbuild.builds.create",
    "cloudbuild.builds.get",
    "run.operations.get",
    "run.revisions.get",
    "run.revisions.list",
    "run.services.create",
    "run.services.get",
    "run.services.getIamPolicy",
    "run.services.list",
    "run.services.setIamPolicy",
    "run.services.update",
    "serviceusage.services.use",
  ]
}

resource "google_project_iam_member" "cloud_build_deployer" {
  count   = var.enable_foundation ? 1 : 0
  project = var.project_id
  role    = google_project_iam_custom_role.cloud_build_deployer[0].name
  member  = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}

resource "google_project_iam_custom_role" "cloud_build_bucket_operator" {
  count       = var.enable_foundation ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoCloudBuildBucketOperator"
  title       = "EspaciGo Cloud Build bucket operator"
  description = "Access only to the dedicated Cloud Build staging/lock bucket."
  stage       = "GA"
  permissions = [
    "storage.buckets.get",
    "storage.objects.create",
    "storage.objects.delete",
    "storage.objects.get",
    "storage.objects.list",
  ]
}

resource "google_project_iam_member" "cloud_build_log_writer" {
  count   = var.enable_foundation ? 1 : 0
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}

resource "google_storage_bucket_iam_member" "cloud_build_bucket_operator" {
  count  = var.enable_foundation ? 1 : 0
  bucket = google_storage_bucket.cloud_build[0].name
  role   = google_project_iam_custom_role.cloud_build_bucket_operator[0].name
  member = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}

resource "google_artifact_registry_repository_iam_member" "cloud_build_writer" {
  count      = var.enable_foundation ? 1 : 0
  location   = google_artifact_registry_repository.containers[0].location
  repository = google_artifact_registry_repository.containers[0].name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}

resource "google_service_account_iam_member" "cloud_build_self_act_as" {
  count              = var.enable_foundation ? 1 : 0
  service_account_id = google_service_account.cloud_build_deployer[0].name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}

resource "google_service_account_iam_member" "cloud_build_runtime_act_as" {
  for_each = var.enable_foundation ? toset(["api", "public-web", "private-web"]) : toset([])

  service_account_id = google_service_account.runtime[each.key].name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.cloud_build_deployer[0].email}"
}

resource "google_service_account" "gcp06_job" {
  for_each     = var.enable_foundation && var.enable_gcp06_jobs_reviewed ? toset(compact([var.enable_gcp06_admin_bootstrap_job ? "admin-bootstrap" : "", var.enable_gcp06_test_access_job ? "test-access" : "", var.enable_gcp06_sql_inspector_job ? "sql-inspector" : ""])) : toset([])
  project      = var.project_id
  account_id   = "espacigo-gcp06-${each.key}"
  display_name = "EspaciGo GCP-06 candidate ${each.key} one-shot job"
  description  = "Run-scoped private candidate job; never used by production or persistent services."

  depends_on = [google_project_service.required]
}

resource "google_service_account" "mailpit" {
  count        = var.enable_mailpit ? 1 : 0
  project      = var.project_id
  account_id   = "espacigo-test-mailpit"
  display_name = "EspaciGo private Mailpit test sink"
  description  = "Read-only pull identity for the pinned Mailpit image; no SQL, application secrets, or relay permissions."

  depends_on = [google_project_service.required]
}

resource "google_iam_workload_identity_pool" "github" {
  count                     = var.enable_foundation ? 1 : 0
  workload_identity_pool_id = "espacigo-testprod-github"
  display_name              = "EspaciGo testprod GitHub Actions"
  description               = "Restricted to the EspaciGo repository main branch."
  disabled                  = false

  depends_on = [google_project_service.required]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  count                              = var.enable_foundation ? 1 : 0
  workload_identity_pool_id          = google_iam_workload_identity_pool.github[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "espacigo-main"
  display_name                       = "EspaciGo main branch"
  attribute_mapping = {
    "google.subject"                = "assertion.sub"
    "attribute.repository"          = "assertion.repository"
    "attribute.repository_id"       = "assertion.repository_id"
    "attribute.repository_owner_id" = "assertion.repository_owner_id"
    "attribute.ref"                 = "assertion.ref"
    "attribute.workflow_ref"        = "assertion.workflow_ref"
    "attribute.environment"         = "assertion.environment"
  }
  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_repository_owner_id}' && assertion.ref == 'refs/heads/main' && (assertion.workflow_ref == '${var.github_repository}/.github/workflows/application-release.yml@refs/heads/main' || assertion.workflow_ref == '${var.github_repository}/.github/workflows/application-rollout.yml@refs/heads/main') && assertion.environment == 'testprod'"
  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  depends_on = [google_project_service.required]
}

resource "google_service_account_iam_member" "github_deployer" {
  count              = var.enable_foundation ? 1 : 0
  service_account_id = google_service_account.runtime["deployer"].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github[0].name}/attribute.repository_id/${var.github_repository_id}"

  depends_on = [google_project_service.required]
}

resource "google_project_iam_custom_role" "deployer_cloud_run_service_updater" {
  count       = var.enable_foundation ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoCloudRunServiceUpdater"
  title       = "EspaciGo Cloud Run service updater"
  description = "Read and update existing Cloud Run services only; no create, delete, Jobs, or IAM policy permissions."
  stage       = "GA"
  permissions = [
    "run.services.get",
    "run.services.update",
    "run.operations.get",
  ]
}

resource "google_project_iam_custom_role" "terraform_state_object_access" {
  count       = var.enable_foundation ? 1 : 0
  project     = var.project_id
  role_id     = "espacigoTerraformStateObjectAccess"
  title       = "EspaciGo Terraform state object access"
  description = "Read/write/list only the canonical testprod/gcp Terraform state and lock objects."
  stage       = "GA"
  permissions = [
    "storage.objects.create",
    "storage.objects.delete",
    "storage.objects.get",
    "storage.objects.list",
  ]
}

resource "google_storage_managed_folder" "terraform_state" {
  count         = var.enable_foundation ? 1 : 0
  bucket        = var.terraform_state_bucket
  name          = "espacigo/testprod/gcp/"
  force_destroy = false
}

resource "google_storage_managed_folder_iam_member" "deployer_terraform_state" {
  count          = var.enable_foundation ? 1 : 0
  bucket         = google_storage_managed_folder.terraform_state[0].bucket
  managed_folder = google_storage_managed_folder.terraform_state[0].name
  role           = "projects/${var.project_id}/roles/${google_project_iam_custom_role.terraform_state_object_access[0].role_id}"
  member         = "serviceAccount:${google_service_account.runtime["deployer"].email}"
}

resource "google_artifact_registry_repository" "containers" {
  count         = var.enable_foundation ? 1 : 0
  location      = var.region
  repository_id = "espacigo-testprod"
  description   = "Digest-pinned EspaciGo testprod images. CI creates and uploads images."
  format        = "DOCKER"
  mode          = "STANDARD_REPOSITORY"

  depends_on = [google_project_service.required]
}

resource "google_artifact_registry_repository" "mailpit_dockerhub" {
  count         = var.enable_mailpit ? 1 : 0
  location      = var.region
  repository_id = "espacigo-testprod-dockerhub"
  description   = "Read-through cache for the exact public Mailpit image digest used by Compose."
  format        = "DOCKER"
  mode          = "REMOTE_REPOSITORY"

  remote_repository_config {
    description = "Public Docker Hub images used by the private test sink."
    docker_repository {
      public_repository = "DOCKER_HUB"
    }
  }

  depends_on = [google_project_service.required]
}

resource "google_artifact_registry_repository_iam_member" "mailpit_reader" {
  count      = var.enable_mailpit ? 1 : 0
  location   = google_artifact_registry_repository.mailpit_dockerhub[0].location
  repository = google_artifact_registry_repository.mailpit_dockerhub[0].name
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.mailpit[0].email}"
}

data "google_compute_image" "mailpit_cos" {
  count   = var.enable_mailpit && var.mailpit_cos_image == "" ? 1 : 0
  project = "cos-cloud"
  family  = "cos-stable"
}

resource "google_compute_address" "mailpit" {
  count        = var.enable_mailpit ? 1 : 0
  project      = var.project_id
  name         = "espacigo-testprod-mailpit"
  region       = var.region
  address_type = "INTERNAL"
  subnetwork   = google_compute_subnetwork.mailpit[0].id
}

resource "google_compute_disk" "mailpit_data" {
  count   = var.enable_mailpit ? 1 : 0
  project = var.project_id
  name    = "espacigo-testprod-mailpit-data"
  type    = "pd-balanced"
  size    = 10
  zone    = var.mailpit_zone

  lifecycle {
    prevent_destroy = true
  }
}

resource "google_compute_firewall" "mailpit_smtp" {
  count       = var.enable_mailpit ? 1 : 0
  project     = var.project_id
  name        = "espacigo-testprod-mailpit-smtp"
  network     = google_compute_network.testprod[0].name
  description = "Only the dedicated API Direct VPC subnet may submit test mail to Mailpit."
  direction   = "INGRESS"
  priority    = 1000

  source_ranges = [var.api_serverless_subnet_cidr]
  target_tags   = ["espacigo-mailpit"]

  allow {
    protocol = "tcp"
    ports    = ["1025"]
  }
}

resource "google_compute_firewall" "mailpit_ui_iap" {
  count       = var.enable_mailpit ? 1 : 0
  project     = var.project_id
  name        = "espacigo-testprod-mailpit-iap-ui"
  network     = google_compute_network.testprod[0].name
  description = "IAP TCP forwarding only to the private Mailpit UI port."
  direction   = "INGRESS"
  priority    = 1000

  source_ranges = ["35.235.240.0/20"]
  target_tags   = ["espacigo-mailpit"]

  allow {
    protocol = "tcp"
    ports    = ["8025"]
  }
}

resource "google_compute_instance" "mailpit" {
  count                     = var.enable_mailpit ? 1 : 0
  project                   = var.project_id
  name                      = "espacigo-testprod-mailpit"
  zone                      = var.mailpit_zone
  machine_type              = "e2-micro"
  allow_stopping_for_update = true
  tags                      = ["espacigo-mailpit"]
  metadata = {
    enable-oslogin         = "TRUE"
    block-project-ssh-keys = "TRUE"
  }

  boot_disk {
    auto_delete = true
    initialize_params {
      image = var.mailpit_cos_image != "" ? var.mailpit_cos_image : data.google_compute_image.mailpit_cos[0].self_link
      size  = 20
      type  = "pd-balanced"
    }
  }

  attached_disk {
    source      = google_compute_disk.mailpit_data[0].id
    device_name = "mailpit-data"
    mode        = "READ_WRITE"
  }

  network_interface {
    network    = google_compute_network.testprod[0].self_link
    subnetwork = google_compute_subnetwork.mailpit[0].self_link
    network_ip = google_compute_address.mailpit[0].address
  }

  service_account {
    email  = google_service_account.mailpit[0].email
    scopes = ["https://www.googleapis.com/auth/cloud-platform"]
  }

  metadata_startup_script = templatefile("${path.module}/mailpit-startup.sh.tftpl", {
    image    = local.mailpit_image
    registry = "${var.region}-docker.pkg.dev"
  })

  depends_on = [
    google_artifact_registry_repository_iam_member.mailpit_reader,
    google_compute_firewall.mailpit_smtp,
    google_compute_firewall.mailpit_ui_iap,
    google_project_service.required,
  ]
}

resource "google_iap_tunnel_instance_iam_member" "mailpit_ui" {
  for_each = var.enable_mailpit ? toset(var.iap_evaluator_members) : toset([])
  project  = var.project_id
  zone     = google_compute_instance.mailpit[0].zone
  instance = google_compute_instance.mailpit[0].name
  role     = "roles/iap.tunnelResourceAccessor"
  member   = each.value

  condition {
    title       = "mailpit-ui-port-only"
    description = "IAP TCP access is limited to the Mailpit UI port."
    expression  = "destination.port == 8025"
  }
}

resource "google_artifact_registry_repository_iam_member" "deployer_writer" {
  count      = var.enable_foundation ? 1 : 0
  location   = google_artifact_registry_repository.containers[0].location
  repository = google_artifact_registry_repository.containers[0].name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.runtime["deployer"].email}"

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket" "private_files" {
  count                       = var.enable_foundation ? 1 : 0
  project                     = var.project_id
  name                        = "espacigo-test-files-${var.bucket_suffix}"
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  versioning {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required]
}

# Build source staging and rollout coordination are isolated from application
# files and Terraform state in one short-lived, private bucket.
resource "google_storage_bucket" "cloud_build" {
  count                       = var.enable_foundation ? 1 : 0
  project                     = var.project_id
  name                        = "espacigo-testprod-cloud-build-${var.project_number}"
  location                    = var.region
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false

  lifecycle_rule {
    action { type = "Delete" }
    condition {
      age            = 30
      matches_prefix = ["source/"]
    }
  }

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.required]
}

resource "google_storage_bucket_iam_member" "api_files" {
  count  = var.enable_foundation ? 1 : 0
  bucket = google_storage_bucket.private_files[0].name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.runtime["api"].email}"

  depends_on = [google_project_service.required]
}

resource "google_secret_manager_secret" "runtime" {
  for_each  = local.runtime_secret_ids
  project   = var.project_id
  secret_id = "espacigo-test-${each.key}"
  replication {
    auto {}
  }
  deletion_policy = "DISABLE"

  depends_on = [google_project_service.required]
}

resource "google_secret_manager_secret_iam_member" "api_secret_access" {
  for_each  = var.enable_foundation ? local.api_secret_access_keys : toset([])
  project   = var.project_id
  secret_id = google_secret_manager_secret.runtime[each.value].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime["api"].email}"

  depends_on = [google_project_service.required]
}

# The V67 runtime password is managed outside this Terraform root. Grant only
# the API runtime identity access to its existing, version-pinned secret.
resource "google_secret_manager_secret_iam_member" "v67_api_secret_access" {
  count     = var.enable_foundation && !local.api_gcp06_candidate_enabled ? 1 : 0
  project   = var.project_id
  secret_id = "espacigo-test-database-password-v67"
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime["api"].email}"

  depends_on = [google_project_service.required]
}

resource "google_secret_manager_secret_iam_member" "migrator_db_access" {
  count     = var.enable_foundation && var.enable_database ? 1 : 0
  project   = var.project_id
  secret_id = google_secret_manager_secret.runtime[local.migrator_database_secret_id].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime["migrator"].email}"

  depends_on = [google_project_service.required]
}

resource "google_project_iam_member" "cloudsql_clients" {
  for_each = var.enable_foundation ? toset(["api", "migrator"]) : toset([])
  project  = var.project_id
  role     = "roles/cloudsql.client"
  member   = "serviceAccount:${google_service_account.runtime[each.key].email}"

  depends_on = [google_project_service.required]
}

resource "google_project_iam_member" "gcp06_job_cloudsql_client" {
  for_each = var.enable_gcp06_jobs_reviewed ? toset(compact([var.enable_gcp06_admin_bootstrap_job ? "admin-bootstrap" : "", var.enable_gcp06_test_access_job ? "test-access" : "", var.enable_gcp06_sql_inspector_job ? "sql-inspector" : ""])) : toset([])
  project  = var.project_id
  role     = "roles/cloudsql.client"
  member   = "serviceAccount:${google_service_account.gcp06_job[each.key].email}"

  condition {
    title       = "${each.key}-only-gcp06-candidate-sql"
    description = "Connector use only for the reviewed GCP-06 candidate instance; inspector grant expires at the separately reviewed timestamp."
    expression  = "resource.name == 'projects/espacigo-testprod/instances/espacigo-gcp06-20261020-gcp06a-pg' && resource.type == 'sqladmin.googleapis.com/Instance'${each.key == "sql-inspector" ? " && request.time < timestamp('${var.gcp06_sql_inspector_iam_expiry}')" : ""}"
  }
}

resource "google_secret_manager_secret_iam_member" "gcp06_job_secret_access" {
  for_each = var.enable_gcp06_jobs_reviewed ? merge(
    var.enable_gcp06_admin_bootstrap_job ? {
      "admin-bootstrap-runtime"    = { job = "admin-bootstrap", secret = local.api_database_secret_key }
      "admin-bootstrap-activation" = { job = "admin-bootstrap", secret = "gcp06-admin-bootstrap-password" }
    } : {},
    var.enable_gcp06_test_access_job ? merge(
      {
        "test-access-db" = { job = "test-access", secret = "gcp06-test-access-db-password" }
      },
      var.gcp06_test_access_mode == "apply" ? {
        "test-access-app-password" = { job = "test-access", secret = "gcp06-test-account-password" }
      } : {},
    ) : {},
    var.enable_gcp06_sql_inspector_job ? {
      "sql-inspector-credential" = { job = "sql-inspector", secret = "gcp06-sql-inspector-credential" }
    } : {},
  ) : {}
  project   = var.project_id
  secret_id = google_secret_manager_secret.runtime[each.value.secret].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.gcp06_job[each.value.job].email}"
}

resource "google_artifact_registry_repository_iam_member" "gcp06_job_image_reader" {
  for_each   = var.enable_gcp06_jobs_reviewed ? toset(compact([var.enable_gcp06_admin_bootstrap_job ? "admin-bootstrap" : "", var.enable_gcp06_test_access_job ? "test-access" : "", var.enable_gcp06_sql_inspector_job ? "sql-inspector" : ""])) : toset([])
  project    = var.project_id
  location   = var.region
  repository = "espacigo-testprod"
  role       = "roles/artifactregistry.reader"
  member     = "serviceAccount:${google_service_account.gcp06_job[each.key].email}"
}

resource "google_cloud_run_v2_job" "gcp06_admin_bootstrap" {
  count               = var.enable_gcp06_jobs_reviewed && var.enable_gcp06_admin_bootstrap_job ? 1 : 0
  project             = var.project_id
  name                = "espacigo-gcp06-owner-bootstrap"
  location            = var.region
  deletion_protection = false
  depends_on = [
    google_project_iam_member.gcp06_job_cloudsql_client,
    google_secret_manager_secret_iam_member.gcp06_job_secret_access,
    google_artifact_registry_repository_iam_member.gcp06_job_image_reader,
  ]

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.gcp06_job["admin-bootstrap"].email
      max_retries     = 0
      timeout         = "300s"

      volumes {
        name = "runtime-password"
        secret {
          secret = google_secret_manager_secret.runtime[local.api_database_secret_key].secret_id
          items {
            version = var.api_gcp06_candidate_password_version
            path    = "password"
            mode    = 256
          }
        }
      }
      volumes {
        name = "activation-password"
        secret {
          secret = google_secret_manager_secret.runtime["gcp06-admin-bootstrap-password"].secret_id
          items {
            version = var.gcp06_admin_bootstrap_password_version
            path    = "password"
            mode    = 256
          }
        }
      }
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.testprod[0].name
          subnetwork = var.enable_mailpit ? google_compute_subnetwork.api_serverless[0].name : google_compute_subnetwork.serverless[0].name
          tags       = ["espacigo-gcp06-admin-bootstrap"]
        }
      }
      containers {
        image = var.gcp06_admin_bootstrap_image
        args  = ["--confirm", "GCP06 BOOTSTRAP 20261020-gcp06a hernan.espinoza.dev@gmail.com"]
        resources { limits = { cpu = "1", memory = "512Mi" } }
        env {
          name  = "GCP06_RUN_ID"
          value = "20261020-gcp06a"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = data.google_sql_database_instance.gcp06_candidate[0].connection_name
        }
        env {
          name  = "DATABASE_NAME"
          value = local.gcp06_candidate_database_name
        }
        env {
          name  = "GCP06_BOOTSTRAP_USER"
          value = var.gcp06_activation_login
        }
        env {
          name  = "GCP06_ACTIVATION_PASSWORD_FILE"
          value = "/var/run/secrets/gcp06-admin-bootstrap/password"
        }
        env {
          name  = "DATABASE_USER"
          value = "espacigo_api_login"
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/var/run/secrets/gcp06-runtime/password"
        }
        volume_mounts {
          name       = "runtime-password"
          mount_path = "/var/run/secrets/gcp06-runtime"
        }
        volume_mounts {
          name       = "activation-password"
          mount_path = "/var/run/secrets/gcp06-admin-bootstrap"
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.api_gcp06_candidate_enabled && var.project_id == "espacigo-testprod" &&
        var.region == "southamerica-west1" && var.api_gcp06_candidate_password_version != "" &&
        can(regex("^[1-9][0-9]*$", var.gcp06_admin_bootstrap_password_version)) &&
        var.gcp06_activation_login != "" && var.gcp06_bootstrap_operator_member != "" &&
        can(regex("^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp06-admin-bootstrap@sha256:[0-9a-f]{64}$", var.gcp06_admin_bootstrap_image))
      )
      error_message = "The bootstrap Job requires the pinned candidate, numeric secret versions, one-time activation login, nominated invoker, and immutable test image digest."
    }
  }
}

resource "google_cloud_run_v2_job" "gcp06_test_access" {
  count               = var.enable_gcp06_jobs_reviewed && var.enable_gcp06_test_access_job ? 1 : 0
  project             = var.project_id
  name                = "espacigo-gcp06-test-access"
  location            = var.region
  deletion_protection = false
  depends_on = [
    google_project_iam_member.gcp06_job_cloudsql_client,
    google_secret_manager_secret_iam_member.gcp06_job_secret_access,
    google_artifact_registry_repository_iam_member.gcp06_job_image_reader,
  ]

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.gcp06_job["test-access"].email
      max_retries     = 0
      timeout         = "1800s"

      volumes {
        name = "sql-password"
        secret {
          secret = google_secret_manager_secret.runtime["gcp06-test-access-db-password"].secret_id
          items {
            version = var.gcp06_test_access_db_password_version
            path    = "password"
            mode    = 256
          }
        }
      }
      dynamic "volumes" {
        for_each = var.gcp06_test_access_mode == "apply" ? [1] : []
        content {
          name = "app-password"
          secret {
            secret = google_secret_manager_secret.runtime["gcp06-test-account-password"].secret_id
            items {
              version = var.gcp06_test_app_password_version
              path    = "password"
              mode    = 256
            }
          }
        }
      }
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.testprod[0].name
          subnetwork = var.enable_mailpit ? google_compute_subnetwork.api_serverless[0].name : google_compute_subnetwork.serverless[0].name
          tags       = ["espacigo-gcp06-test-access"]
        }
      }
      containers {
        image = var.gcp06_test_access_image
        resources { limits = { cpu = "1", memory = "1Gi" } }
        env {
          name  = "GCP06_RUN_ID"
          value = "20261020-gcp06a"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = data.google_sql_database_instance.gcp06_candidate[0].connection_name
        }
        env {
          name  = "DATABASE_NAME"
          value = local.gcp06_candidate_database_name
        }
        env {
          name  = "GCP06_TEST_ACCESS_USER"
          value = var.gcp06_test_access_login
        }
        env {
          name  = "GCP06_TEST_ACCESS_MODE"
          value = var.gcp06_test_access_mode
        }
        env {
          name  = "GCP06_TEST_ACCESS_EXPECTED_COUNT"
          value = tostring(var.gcp06_test_access_expected_count)
        }
        env {
          name  = "GCP06_TEST_ACCESS_EXPECTED_SHA256"
          value = var.gcp06_test_access_expected_sha256
        }
        env {
          name  = "GCP06_TEST_ACCESS_OPERATION_ID"
          value = var.gcp06_test_access_operation_id
        }
        env {
          name  = "GCP06_TEST_ACCESS_CONFIRMATION"
          value = var.gcp06_test_access_confirmation
        }
        env {
          name  = "DATABASE_PASSWORD_FILE"
          value = "/var/run/secrets/gcp06-test-access-db/password"
        }
        env {
          name  = "GCP06_TEST_ACCESS_APP_PASSWORD_FILE"
          value = "/var/run/secrets/gcp06-test-account/password"
        }
        volume_mounts {
          name       = "sql-password"
          mount_path = "/var/run/secrets/gcp06-test-access-db"
        }
        dynamic "volume_mounts" {
          for_each = var.gcp06_test_access_mode == "apply" ? [1] : []
          content {
            name       = "app-password"
            mount_path = "/var/run/secrets/gcp06-test-account"
          }
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        local.api_gcp06_candidate_enabled && var.project_id == "espacigo-testprod" &&
        var.region == "southamerica-west1" && can(regex("^[1-9][0-9]*$", var.gcp06_test_access_db_password_version)) &&
        var.gcp06_test_access_login != "" && var.gcp06_test_access_operator_member != "" &&
        can(regex("^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp06-test-access@sha256:[0-9a-f]{64}$", var.gcp06_test_access_image)) &&
        (var.gcp06_test_access_mode == "preview" || (var.gcp06_test_access_mode == "apply" &&
          can(regex("^[1-9][0-9]*$", var.gcp06_test_app_password_version)) && var.gcp06_test_access_expected_count > 0 &&
          can(regex("^[0-9a-f]{64}$", var.gcp06_test_access_expected_sha256)) &&
          can(regex("^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$", var.gcp06_test_access_operation_id)) &&
        var.gcp06_test_access_confirmation == "GCP06 APPLY 20261020-gcp06a ${var.gcp06_test_access_expected_count} ${var.gcp06_test_access_expected_sha256} ${var.gcp06_test_access_operation_id}"))
      )
      error_message = "The reset Job must be preview-only unless an exact preview cohort, numeric Secret Manager version, unique operation ID, and confirmation are pinned."
    }
  }
}

resource "google_cloud_run_v2_job_iam_member" "gcp06_bootstrap_invoker" {
  count    = var.enable_gcp06_jobs_reviewed && var.enable_gcp06_admin_bootstrap_job ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.gcp06_admin_bootstrap[0].name
  role     = "roles/run.invoker"
  member   = var.gcp06_bootstrap_operator_member
}

resource "google_cloud_run_v2_job_iam_member" "gcp06_test_access_invoker" {
  count    = var.enable_gcp06_jobs_reviewed && var.enable_gcp06_test_access_job ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.gcp06_test_access[0].name
  role     = "roles/run.invoker"
  member   = var.gcp06_test_access_operator_member
}

resource "google_cloud_run_v2_job" "gcp06_sql_inspector" {
  count               = var.enable_gcp06_jobs_reviewed && var.enable_gcp06_sql_inspector_job ? 1 : 0
  project             = var.project_id
  name                = "espacigo-gcp06-sql-inspector"
  location            = var.region
  deletion_protection = false
  depends_on = [
    google_project_iam_member.gcp06_job_cloudsql_client,
    google_secret_manager_secret_iam_member.gcp06_job_secret_access,
    google_artifact_registry_repository_iam_member.gcp06_job_image_reader,
  ]

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.gcp06_job["sql-inspector"].email
      max_retries     = 0
      timeout         = "600s"
      volumes {
        name = "inspector-credential"
        secret {
          secret = google_secret_manager_secret.runtime["gcp06-sql-inspector-credential"].secret_id
          items {
            version = var.gcp06_sql_inspector_secret_version
            path    = "credentials.json"
            mode    = 256
          }
        }
      }
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.testprod[0].name
          subnetwork = google_compute_subnetwork.serverless[0].name
          tags       = ["espacigo-gcp06-sql-inspector"]
        }
      }
      containers {
        image = var.gcp06_sql_inspector_image
        resources { limits = { cpu = "1", memory = "512Mi" } }
        env {
          name  = "GCP06_RUN_ID"
          value = "20261020-gcp06a"
        }
        env {
          name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
          value = data.google_sql_database_instance.gcp06_candidate[0].connection_name
        }
        env {
          name  = "DATABASE_NAME"
          value = local.gcp06_candidate_database_name
        }
        env {
          name  = "GCP06_SQL_INSPECTOR_CREDENTIAL_FILE"
          value = "/var/run/secrets/gcp06-sql-inspector/credentials.json"
        }
        volume_mounts {
          name       = "inspector-credential"
          mount_path = "/var/run/secrets/gcp06-sql-inspector"
        }
      }
    }
  }

  lifecycle {
    precondition {
      condition = (
        length(data.google_sql_database_instance.gcp06_candidate) == 1 && var.project_id == "espacigo-testprod" && var.region == "southamerica-west1" &&
        can(regex("^[1-9][0-9]*$", var.gcp06_sql_inspector_secret_version)) &&
        can(regex("^southamerica-west1-docker\\.pkg\\.dev/espacigo-testprod/espacigo-testprod/gcp06-sql-inspect@sha256:[0-9a-f]{64}$", var.gcp06_sql_inspector_image)) &&
        var.gcp06_sql_inspector_operator_member != "" && can(timecmp(var.gcp06_sql_inspector_iam_expiry, plantimestamp()) > 0)
      )
      error_message = "The inspector requires the exact candidate, numeric credential version, immutable image, one named invoker, and a future IAM expiry."
    }
  }
}

resource "google_cloud_run_v2_job_iam_member" "gcp06_sql_inspector_invoker" {
  count    = var.enable_gcp06_jobs_reviewed && var.enable_gcp06_sql_inspector_job ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.gcp06_sql_inspector[0].name
  role     = "roles/run.invoker"
  member   = var.gcp06_sql_inspector_operator_member
}

resource "google_compute_region_network_endpoint_group" "web" {
  for_each              = var.enable_edge ? local.web_services : {}
  name                  = "${each.key}-web-serverless"
  region                = var.region
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = each.value.name
  }
}

resource "google_compute_backend_service" "web" {
  for_each              = var.enable_edge ? local.web_services : {}
  name                  = "espacigo-testprod-${each.key}-web"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  backend {
    group = google_compute_region_network_endpoint_group.web[each.key].id
  }
  security_policy = each.key == "public" ? google_compute_security_policy.public_edge[0].id : null
}

resource "google_compute_security_policy" "public_edge" {
  count = var.enable_edge ? 1 : 0
  name  = "espacigo-testprod-public-edge"
  type  = "CLOUD_ARMOR"

  rule {
    action   = "throttle"
    priority = 100
    match {
      expr {
        expression = "request.path == '/api/v1/public/spaces'"
      }
    }
    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"
      enforce_on_key = "IP"
      rate_limit_threshold {
        count        = var.public_catalog_requests_per_minute
        interval_sec = 60
      }
    }
  }

  rule {
    action   = "throttle"
    priority = 200
    match {
      expr {
        expression = "request.path.startsWith('/api/v1/public/') || request.path.startsWith('/public/spaces/') || request.path.startsWith('/espacios/') || request.path == '/explorar' || request.path == '/explorar/' || request.path == '/sitemap.xml' || request.path == '/health/ready'"
      }
    }
    rate_limit_options {
      conform_action = "allow"
      exceed_action  = "deny(429)"
      enforce_on_key = "IP"
      rate_limit_threshold {
        count        = var.public_other_requests_per_minute
        interval_sec = 60
      }
    }
  }

  rule {
    action   = "allow"
    priority = 2147483647
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
  }

  depends_on = [google_project_service.required]
}

resource "google_compute_global_address" "edge" {
  count = var.enable_edge ? 1 : 0
  name  = "espacigo-testprod-edge-ipv4"

  depends_on = [google_project_service.required]
}

resource "google_compute_managed_ssl_certificate" "edge" {
  count = var.enable_edge ? 1 : 0
  name  = "espacigo-testprod-edge-cert"
  managed {
    domains = [var.public_hostname, var.private_hostname]
  }

  depends_on = [google_project_service.required]
}

resource "google_compute_url_map" "edge" {
  count           = var.enable_edge ? 1 : 0
  name            = "espacigo-testprod-edge"
  default_service = google_compute_backend_service.web["public"].id

  host_rule {
    hosts        = [var.public_hostname]
    path_matcher = "public-host"
  }
  host_rule {
    hosts        = [var.private_hostname]
    path_matcher = "private-host"
  }
  path_matcher {
    name            = "public-host"
    default_service = google_compute_backend_service.web["public"].id
  }
  path_matcher {
    name            = "private-host"
    default_service = google_compute_backend_service.web["private"].id
  }
}

resource "google_compute_target_https_proxy" "edge" {
  count            = var.enable_edge ? 1 : 0
  name             = "espacigo-testprod-edge-https"
  url_map          = google_compute_url_map.edge[0].id
  ssl_certificates = [google_compute_managed_ssl_certificate.edge[0].id]
}

resource "google_compute_global_forwarding_rule" "https" {
  count                 = var.enable_edge ? 1 : 0
  name                  = "espacigo-testprod-edge-https"
  ip_address            = google_compute_global_address.edge[0].address
  target                = google_compute_target_https_proxy.edge[0].id
  port_range            = "443"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL_MANAGED"

  depends_on = [google_project_service.required]
}
