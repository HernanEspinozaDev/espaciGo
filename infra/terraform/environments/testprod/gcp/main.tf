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
  runtime_secret_ids = var.enable_foundation ? setunion(
    toset(["database-password", "contract-encryption-key", "payment-webhook-secret"]),
    var.enable_database ? toset(["database-migrator-password"]) : toset([]),
  ) : toset([])
  migrator_database_secret_id = var.enable_database ? "database-migrator-password" : "database-password"
  project_enabled_apis = toset([
    "artifactregistry.googleapis.com",
    "cloudresourcemanager.googleapis.com",
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
  web_services = var.enable_services ? {
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
  } : {}
}

resource "google_project_service" "required" {
  for_each                   = var.enable_foundation ? local.project_enabled_apis : toset([])
  project                    = var.project_id
  service                    = each.value
  disable_dependent_services = false
  disable_on_destroy         = false
}

resource "google_project_service_identity" "iap" {
  provider = google-beta
  count    = var.enable_services ? 1 : 0
  project  = var.project_id
  service  = "iap.googleapis.com"

  depends_on = [google_project_service.required]
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

resource "google_compute_global_address" "private_services" {
  count         = var.enable_database ? 1 : 0
  name          = "espacigo-testprod-private-services"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  ip_version    = "IPV4"
  prefix_length = 16
  network       = google_compute_network.testprod[0].id
}

resource "google_service_networking_connection" "private_services" {
  count                   = var.enable_database ? 1 : 0
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
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"
  }
  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.ref == 'refs/heads/main'"
  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  depends_on = [google_project_service.required]
}

resource "google_service_account_iam_member" "github_deployer" {
  count              = var.enable_foundation ? 1 : 0
  service_account_id = google_service_account.runtime["deployer"].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github[0].name}/attribute.repository/${var.github_repository}"

  depends_on = [google_project_service.required]
}

resource "google_project_iam_member" "deployer_run" {
  count   = var.enable_foundation ? 1 : 0
  project = var.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.runtime["deployer"].email}"

  depends_on = [google_project_service.required]
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
  for_each  = var.enable_foundation ? toset(["database-password", "contract-encryption-key", "payment-webhook-secret"]) : toset([])
  project   = var.project_id
  secret_id = google_secret_manager_secret.runtime[each.value].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.runtime["api"].email}"

  depends_on = [google_project_service.required]
}

resource "google_secret_manager_secret_iam_member" "migrator_db_access" {
  count     = var.enable_foundation ? 1 : 0
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

resource "google_sql_database_instance" "postgres" {
  count               = var.enable_database ? 1 : 0
  project             = var.project_id
  name                = "espacigo-testprod-pg"
  region              = var.region
  database_version    = "POSTGRES_18"
  deletion_protection = true
  deletion_policy     = "PREVENT"

  settings {
    edition                     = "ENTERPRISE"
    deletion_protection_enabled = true
    tier                        = var.cloudsql_tier
    connector_enforcement       = "REQUIRED"
    disk_type                   = "HYPERDISK_BALANCED"
    disk_size                   = var.cloudsql_disk_gib
    disk_autoresize             = true
    disk_autoresize_limit       = var.cloudsql_disk_limit_gib
    availability_type           = "ZONAL"
    user_labels = {
      application = "espacigo"
      environment = "testprod"
      managed-by  = "terraform"
    }

    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.testprod[0].id
      ssl_mode        = "ENCRYPTED_ONLY"
    }

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      transaction_log_retention_days = 7
      backup_retention_settings {
        retained_backups = 7
        retention_unit   = "COUNT"
      }
    }
  }

  depends_on = [google_service_networking_connection.private_services]
}

resource "google_sql_database" "application" {
  count           = var.enable_database ? 1 : 0
  project         = var.project_id
  name            = "espacigo"
  instance        = google_sql_database_instance.postgres[0].name
  charset         = "UTF8"
  collation       = "en_US.UTF8"
  deletion_policy = "PREVENT"
}

resource "google_cloud_run_v2_service" "api" {
  count               = var.enable_services ? 1 : 0
  project             = var.project_id
  name                = var.api_service_name
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_INTERNAL_ONLY"
  deletion_protection = true

  template {
    service_account                  = google_service_account.runtime["api"].email
    max_instance_request_concurrency = 80
    scaling {
      min_instance_count = 1
      max_instance_count = var.api_max_instances
    }
    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.testprod[0].name
        subnetwork = google_compute_subnetwork.serverless[0].name
      }
    }
    volumes {
      name = "database-secret"
      secret {
        secret = google_secret_manager_secret.runtime["database-password"].secret_id
        items {
          version = "latest"
          path    = "password"
        }
      }
    }
    volumes {
      name = "contract-secret"
      secret {
        secret = google_secret_manager_secret.runtime["contract-encryption-key"].secret_id
        items {
          version = "latest"
          path    = "key"
        }
      }
    }
    volumes {
      name = "payment-secret"
      secret {
        secret = google_secret_manager_secret.runtime["payment-webhook-secret"].secret_id
        items {
          version = "latest"
          path    = "webhook"
        }
      }
    }
    containers {
      image = var.api_image
      resources {
        cpu_idle = false
        limits   = { cpu = "1", memory = "1Gi" }
      }
      volume_mounts {
        name       = "database-secret"
        mount_path = "/var/secrets/database"
      }
      volume_mounts {
        name       = "contract-secret"
        mount_path = "/var/secrets/contract"
      }
      volume_mounts {
        name       = "payment-secret"
        mount_path = "/var/secrets/payment"
      }
      env {
        name  = "APP_PROFILE"
        value = "cloud-test"
      }
      env {
        name  = "DATABASE_CONNECTION_MODE"
        value = "cloudsql"
      }
      env {
        name  = "CLOUD_SQL_INSTANCE_CONNECTION_NAME"
        value = google_sql_database_instance.postgres[0].connection_name
      }
      env {
        name  = "DATABASE_NAME"
        value = google_sql_database.application[0].name
      }
      env {
        name  = "DATABASE_PASSWORD_FILE"
        value = "/var/secrets/database/password"
      }
      env {
        name  = "LOCAL_CONTRACT_ENCRYPTION_KEY_FILE"
        value = "/var/secrets/contract/key"
      }
      env {
        name  = "LOCAL_PAYMENT_WEBHOOK_SECRET_FILE"
        value = "/var/secrets/payment/webhook"
      }
      env {
        name  = "DATABASE_USER"
        value = "espacigo_runtime"
      }
      env {
        name  = "DATABASE_POOL_MAX_CONNS"
        value = "10"
      }
      env {
        name  = "CLOUD_SQL_PRIVATE_IP"
        value = "true"
      }
      env {
        name  = "LOCAL_BOOKING_TRIAL"
        value = "1"
      }
      env {
        name  = "LOCAL_SMTP_ADDR"
        value = var.mailpit_private_address
      }
      env {
        name  = "M03_EVIDENCE_DIR"
        value = "/var/lib/espacigo/evidence"
      }
      env {
        name  = "CLOUD_STORAGE_BUCKET"
        value = google_storage_bucket.private_files[0].name
      }
      env {
        name  = "BLOB_STORE_BACKEND"
        value = "gcs"
      }
    }
  }

  depends_on = [
    google_project_service.required,
    google_storage_bucket_iam_member.api_files,
    google_secret_manager_secret_iam_member.api_secret_access,
    google_project_iam_member.cloudsql_clients,
  ]
}

resource "google_cloud_run_v2_service" "web" {
  for_each            = local.web_services
  project             = var.project_id
  name                = each.value.name
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
  iap_enabled         = each.value.iap_enabled
  deletion_protection = true

  template {
    service_account                  = google_service_account.runtime[each.key == "public" ? "public-web" : "private-web"].email
    max_instance_request_concurrency = 40
    scaling {
      min_instance_count = 0
      max_instance_count = var.web_max_instances
    }
    vpc_access {
      egress = "ALL_TRAFFIC"
      network_interfaces {
        network    = google_compute_network.testprod[0].name
        subnetwork = google_compute_subnetwork.serverless[0].name
      }
    }
    containers {
      image = var.frontend_image
      resources {
        limits = { cpu = "1", memory = "1Gi" }
      }
      env {
        name  = "FRONTEND_ENTRY_MODE"
        value = each.value.entry_mode
      }
      env {
        name  = "API_INTERNAL_BASE_URL"
        value = "${google_cloud_run_v2_service.api[0].uri}/api/v1"
      }
      env {
        name  = "API_PUBLIC_BASE_URL"
        value = "/api/v1"
      }
      env {
        name  = "API_SERVICE_AUDIENCE"
        value = google_cloud_run_v2_service.api[0].uri
      }
      env {
        name  = "FRONTEND_PUBLIC_ORIGIN"
        value = each.value.public_host
      }
      env {
        name  = "PRIVATE_DEMO_ORIGIN"
        value = "https://${var.private_hostname}"
      }
      env {
        name  = "IAP_JWT_AUDIENCE"
        value = each.key == "private" ? "/projects/${var.project_number}/locations/${var.region}/services/${var.private_service_name}" : ""
      }
    }
  }

  depends_on = [
    google_project_service.required,
    google_cloud_run_v2_service_iam_member.api_invoker,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "api_invoker" {
  for_each = var.enable_services ? toset(["public", "private"]) : toset([])
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.api[0].name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.runtime[each.key == "public" ? "public-web" : "private-web"].email}"
}

resource "google_cloud_run_v2_service_iam_member" "public_web_invoker" {
  count    = var.enable_services ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.web["public"].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

resource "google_cloud_run_v2_service_iam_member" "private_iap_invoker" {
  count    = var.enable_services ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.web["private"].name
  role     = "roles/run.invoker"
  member   = google_project_service_identity.iap[0].member
}

resource "google_service_account_iam_member" "deployer_act_as_runtime" {
  for_each           = var.enable_foundation ? toset(["api", "public-web", "private-web", "migrator"]) : toset([])
  service_account_id = google_service_account.runtime[each.value].name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.runtime["deployer"].email}"

  depends_on = [google_project_service.required]
}

resource "google_iap_web_cloud_run_service_iam_member" "private_evaluators" {
  for_each               = var.enable_services ? toset(var.iap_evaluator_members) : toset([])
  project                = var.project_id
  location               = var.region
  cloud_run_service_name = google_cloud_run_v2_service.web["private"].name
  role                   = "roles/iap.httpsResourceAccessor"
  member                 = each.value
}

resource "google_compute_region_network_endpoint_group" "web" {
  for_each              = var.enable_edge && var.enable_services ? local.web_services : {}
  name                  = "${each.key}-web-serverless"
  region                = var.region
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = google_cloud_run_v2_service.web[each.key].name
  }
}

resource "google_compute_backend_service" "web" {
  for_each              = var.enable_edge && var.enable_services ? local.web_services : {}
  name                  = "espacigo-testprod-${each.key}-web"
  protocol              = "HTTP"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  backend {
    group = google_compute_region_network_endpoint_group.web[each.key].id
  }
  security_policy = each.key == "public" ? google_compute_security_policy.public_edge[0].id : null
}

resource "google_compute_security_policy" "public_edge" {
  count = var.enable_edge && var.enable_services ? 1 : 0
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
  count = var.enable_edge && var.enable_services ? 1 : 0
  name  = "espacigo-testprod-edge-ipv4"

  depends_on = [google_project_service.required]
}

resource "google_compute_managed_ssl_certificate" "edge" {
  count = var.enable_edge && var.enable_services ? 1 : 0
  name  = "espacigo-testprod-edge-cert"
  managed {
    domains = [var.public_hostname, var.private_hostname]
  }

  depends_on = [google_project_service.required]
}

resource "google_compute_url_map" "edge" {
  count           = var.enable_edge && var.enable_services ? 1 : 0
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
  count            = var.enable_edge && var.enable_services ? 1 : 0
  name             = "espacigo-testprod-edge-https"
  url_map          = google_compute_url_map.edge[0].id
  ssl_certificates = [google_compute_managed_ssl_certificate.edge[0].id]
}

resource "google_compute_global_forwarding_rule" "https" {
  count                 = var.enable_edge && var.enable_services ? 1 : 0
  name                  = "espacigo-testprod-edge-https"
  ip_address            = google_compute_global_address.edge[0].address
  target                = google_compute_target_https_proxy.edge[0].id
  port_range            = "443"
  ip_protocol           = "TCP"
  load_balancing_scheme = "EXTERNAL_MANAGED"

  depends_on = [
    google_project_service.required,
    google_cloud_run_v2_service_iam_member.public_web_invoker,
    google_cloud_run_v2_service_iam_member.private_iap_invoker,
  ]
}
