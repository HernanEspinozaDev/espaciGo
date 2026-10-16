mock_provider "google" {
  override_during = plan

  mock_resource "google_cloud_run_v2_service" {
    defaults = {
      uri = "https://espacigo-test.run.app"
    }
  }

  mock_resource "google_compute_security_policy" {
    defaults = {
      id = "projects/offline-test-project/global/securityPolicies/public"
    }
  }

  mock_resource "google_compute_backend_service" {
    defaults = {
      id = "projects/offline-test-project/global/backendServices/mock"
    }
  }
}

run "foundation_plan_is_gated_and_scoped" {
  command = plan

  variables {
    project_id                 = "offline-test-project"
    project_number             = "123456789012"
    enable_foundation          = true
    confirm_project_inventory  = true
    confirm_cost_policy_review = true
    bucket_suffix              = "offline-test"
  }

  assert {
    condition     = length(google_project_service.required) == 11
    error_message = "Foundation should plan only its explicit minimum API set."
  }
  assert {
    condition     = length(google_compute_network.testprod) == 1 && length(google_compute_subnetwork.serverless) == 1
    error_message = "Foundation should create one dedicated VPC and serverless subnet."
  }
}

run "foundation_rejects_missing_human_gates" {
  command = plan

  variables {
    project_id        = "offline-test-project"
    project_number    = "123456789012"
    enable_foundation = true
    bucket_suffix     = "offline-test"
  }

  expect_failures = [var.enable_foundation]
}

run "database_plan_is_private_and_protected" {
  command = plan

  variables {
    project_id                 = "offline-test-project"
    project_number             = "123456789012"
    enable_foundation          = true
    enable_database            = true
    confirm_project_inventory  = true
    confirm_cost_policy_review = true
    bucket_suffix              = "offline-test"
  }

  assert {
    condition     = google_sql_database_instance.postgres[0].settings[0].ip_configuration[0].ipv4_enabled == false
    error_message = "Cloud SQL must not receive a public IPv4 address."
  }
  assert {
    condition     = google_sql_database_instance.postgres[0].deletion_protection == true
    error_message = "Cloud SQL deletion protection must remain enabled."
  }
}

run "services_and_edge_preserve_runtime_contracts" {
  command = plan

  variables {
    project_id                      = "offline-test-project"
    project_number                  = "123456789012"
    enable_foundation               = true
    enable_database                 = true
    enable_services                 = true
    enable_edge                     = true
    confirm_project_inventory       = true
    confirm_cost_policy_review      = true
    acknowledge_gcp04_runtime_gates = true
    bucket_suffix                   = "offline-test"
    api_image                       = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                  = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address         = "10.80.0.10:1025"
    confirm_pga_iap_key_preflight   = true
  }

  assert {
    condition     = google_cloud_run_v2_service.api[0].template[0].containers[0].resources[0].cpu_idle == false
    error_message = "API workers need explicitly allocated CPU while their instance is running."
  }
  assert {
    condition     = alltrue([for service in values(google_cloud_run_v2_service.web) : service.template[0].vpc_access[0].egress == "ALL_TRAFFIC"])
    error_message = "SSR-to-API requests must traverse Direct VPC egress for internal ingress recognition."
  }
  assert {
    condition     = output.api_internal_base_url == "https://espacigo-test.run.app/api/v1"
    error_message = "API_INTERNAL_BASE_URL must satisfy configuredApiBase's /api/v1 contract."
  }
  assert {
    condition     = output.api_service_audience == "https://espacigo-test.run.app"
    error_message = "The IAM ID-token audience must remain the API service origin without a path."
  }
  assert {
    condition     = one([for env in google_cloud_run_v2_service.web["public"].template[0].containers[0].env : env.value if env.name == "API_INTERNAL_BASE_URL"]) == output.api_internal_base_url
    error_message = "The deployed SSR environment and exported API base URL must match."
  }
  assert {
    condition     = one([for env in google_cloud_run_v2_service.web["public"].template[0].containers[0].env : env.value if env.name == "API_SERVICE_AUDIENCE"]) == output.api_service_audience
    error_message = "The deployed SSR audience and exported API audience must match."
  }
  assert {
    condition     = google_cloud_run_v2_service.web["private"].iap_enabled == true && google_cloud_run_v2_service.web["private"].ingress == "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
    error_message = "The private entry must retain native Cloud Run IAP and load-balancer-only ingress."
  }
  assert {
    condition     = google_compute_subnetwork.serverless[0].private_ip_google_access == true && google_cloud_run_v2_service.web["private"].template[0].vpc_access[0].egress == "ALL_TRAFFIC"
    error_message = "The private SSR's IAP key download must use its Direct VPC egress subnet with Private Google Access."
  }
  assert {
    condition     = google_compute_network.testprod[0].delete_default_routes_on_create == false
    error_message = "The VPC must retain its default-internet-gateway route required for Private Google Access to Google service destinations."
  }
  assert {
    condition     = google_compute_backend_service.web["public"].security_policy == google_compute_security_policy.public_edge[0].id && google_compute_backend_service.web["private"].security_policy == null
    error_message = "Cloud Armor must be attached only to the public backend."
  }
  assert {
    condition     = length(google_compute_security_policy.public_edge[0].rule) == 3 && contains([for rule in google_compute_security_policy.public_edge[0].rule : rule.priority], 100) && contains([for rule in google_compute_security_policy.public_edge[0].rule : rule.priority], 200)
    error_message = "The public policy must keep separate priority-ordered catalog and general public route throttles plus its terminal allow rule."
  }
  assert {
    condition     = one([for rule in google_compute_security_policy.public_edge[0].rule : rule.rate_limit_options[0].rate_limit_threshold[0].count if rule.priority == 100]) == var.public_catalog_requests_per_minute && one([for rule in google_compute_security_policy.public_edge[0].rule : rule.rate_limit_options[0].rate_limit_threshold[0].count if rule.priority == 200]) == var.public_other_requests_per_minute
    error_message = "Each rate-limit rule must use its own configurable per-IP threshold."
  }
  assert {
    condition     = strcontains(one([for rule in google_compute_security_policy.public_edge[0].rule : rule.match[0].expr[0].expression if rule.priority == 200]), "/sitemap.xml") && strcontains(one([for rule in google_compute_security_policy.public_edge[0].rule : rule.match[0].expr[0].expression if rule.priority == 200]), "/health/ready") && strcontains(one([for rule in google_compute_security_policy.public_edge[0].rule : rule.match[0].expr[0].expression if rule.priority == 200]), "/public/spaces/") && strcontains(one([for rule in google_compute_security_policy.public_edge[0].rule : rule.match[0].expr[0].expression if rule.priority == 200]), "/explorar") && strcontains(one([for rule in google_compute_security_policy.public_edge[0].rule : rule.match[0].expr[0].expression if rule.priority == 200]), "/espacios/")
    error_message = "Public SSR, image, sitemap, readiness and API paths must be rate-limited by the general public rule."
  }
}

run "services_reject_unverified_iap_key_egress" {
  command = plan

  variables {
    project_id                      = "offline-test-project"
    project_number                  = "123456789012"
    enable_foundation               = true
    enable_database                 = true
    enable_services                 = true
    confirm_project_inventory       = true
    confirm_cost_policy_review      = true
    acknowledge_gcp04_runtime_gates = true
    bucket_suffix                   = "offline-test"
    api_image                       = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                  = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address         = "10.80.0.10:1025"
  }

  expect_failures = [var.enable_services]
}
