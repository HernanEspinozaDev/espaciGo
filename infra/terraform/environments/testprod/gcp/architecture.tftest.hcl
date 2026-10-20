mock_provider "google" {
  override_during = plan

  mock_resource "google_cloud_run_v2_service" {
    defaults = {
      uri = "https://espacigo-test.run.app"
    }
  }

  mock_data "google_compute_image" {
    defaults = {
      self_link = "projects/cos-cloud/global/images/cos-stable-test"
    }
  }

  mock_data "google_sql_database_instance" {
    defaults = {
      connection_name    = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
      database_version   = "POSTGRES_18"
      private_ip_address = "10.189.0.13"
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

mock_provider "google-beta" {
  override_during = plan

  mock_resource "google_project_service_identity" {
    defaults = {
      email  = "service-123456789012@gcp-sa-iap.iam.gserviceaccount.com"
      member = "serviceAccount:service-123456789012@gcp-sa-iap.iam.gserviceaccount.com"
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
    condition     = length(google_project_service.required) == 13
    error_message = "Foundation should plan only its explicit minimum API set."
  }
  assert {
    condition     = length(google_compute_network.testprod) == 1 && length(google_compute_subnetwork.serverless) == 1
    error_message = "Foundation should create one dedicated VPC and serverless subnet."
  }
  assert {
    condition     = length(google_cloud_run_v2_job.gcp06_admin_bootstrap) == 0 && length(google_cloud_run_v2_job.gcp06_test_access) == 0 && length(google_cloud_run_v2_job.gcp06_sql_inspector) == 0 && length(google_service_account.gcp06_job) == 0
    error_message = "GCP-06 candidate Jobs and their identities must remain absent until their dedicated review gate is enabled."
  }
}

run "sql_inspector_is_private_readonly_and_time_bounded" {
  command = plan
  variables {
    project_id                          = "espacigo-testprod"
    project_number                      = "123456789012"
    enable_foundation                   = true
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    acknowledge_gcp04_runtime_gates     = true
    bucket_suffix                       = "inspector-job"
    enable_gcp06_sql_inspector_job      = true
    enable_gcp06_jobs_reviewed          = true
    gcp06_sql_inspector_image           = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp06-sql-inspect@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
    gcp06_sql_inspector_secret_version  = "7"
    gcp06_sql_inspector_operator_member = "user:operator@example.test"
    gcp06_sql_inspector_iam_expiry      = "2026-11-01T00:00:00Z"
  }
  override_data {
    target = data.google_sql_database_instance.gcp06_candidate[0]
    values = {
      connection_name    = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
      database_version   = "POSTGRES_18"
      private_ip_address = "10.189.0.13"
      region             = "southamerica-west1"
    }
  }
  assert {
    condition = (
      length(google_cloud_run_v2_job.gcp06_sql_inspector) == 1 &&
      google_cloud_run_v2_job.gcp06_sql_inspector[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.gcp06_sql_inspector[0].template[0].template[0].max_retries == 0 &&
      google_service_account.gcp06_job["sql-inspector"].account_id == "espacigo-gcp06-sql-inspector" &&
      contains(local.runtime_secret_ids, "gcp06-sql-inspector-credential")
    )
    error_message = "The catalog inspector must be a one-shot job with its dedicated identity and exact credential secret."
  }
  assert {
    condition = (
      google_project_iam_member.gcp06_job_cloudsql_client["sql-inspector"].condition[0].expression == "resource.name == 'projects/espacigo-testprod/instances/espacigo-gcp06-20261020-gcp06a-pg' && resource.type == 'sqladmin.googleapis.com/Instance' && request.time < timestamp('2026-11-01T00:00:00Z')" &&
      length(google_secret_manager_secret_iam_member.gcp06_job_secret_access) == 1 &&
      google_cloud_run_v2_job.gcp06_sql_inspector[0].template[0].template[0].vpc_access[0].egress == "PRIVATE_RANGES_ONLY"
    )
    error_message = "Inspector permissions must be scoped to the candidate instance and expiration, and execution must use private VPC egress."
  }
  assert {
    condition = alltrue([
      for env in google_cloud_run_v2_job.gcp06_sql_inspector[0].template[0].template[0].containers[0].env :
      !can(regex("(?i)password", env.name))
    ])
    error_message = "The inspector Job must not pass a password in environment variables."
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
    project_id                          = "offline-test-project"
    project_number                      = "123456789012"
    enable_foundation                   = true
    enable_database                     = true
    enable_services                     = true
    api_contract_encryption_key_version = "1"
    enable_edge                         = true
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    acknowledge_gcp04_runtime_gates     = true
    confirm_pga_iap_key_preflight       = true
    bucket_suffix                       = "offline-test"
    api_image                           = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                      = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address             = "10.80.0.10:1025"
    confirm_runtime_secret_versions     = true
    confirm_runtime_database_bootstrap  = true
  }

  assert {
    condition     = google_cloud_run_v2_service.api[0].template[0].containers[0].resources[0].cpu_idle == false
    error_message = "API workers need explicitly allocated CPU while their instance is running."
  }
  assert {
    condition     = one([for env in google_cloud_run_v2_service.api[0].template[0].containers[0].env : env.value if env.name == "LOCAL_VERIFICATION_IP_LIMIT"]) == "20"
    error_message = "The cloud-test API profile must receive a valid local verification quota at startup."
  }
  assert {
    condition     = google_cloud_run_v2_service.api[0].ingress == "INGRESS_TRAFFIC_ALL"
    error_message = "The API run.app URL must be network-reachable for authenticated SSR service-to-service calls; IAM still limits callers."
  }
  assert {
    condition     = alltrue([for service in values(google_cloud_run_v2_service.web) : service.ingress == (var.enable_edge ? "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER" : "INGRESS_TRAFFIC_ALL")])
    error_message = "Web ingress must match the selected run.app or load-balancer stage."
  }
  assert {
    condition     = one([for env in google_cloud_run_v2_service.web["public"].template[0].containers[0].env : env.value if env.name == "NG_ALLOWED_HOSTS"]) == var.public_hostname && one([for env in google_cloud_run_v2_service.web["private"].template[0].containers[0].env : env.value if env.name == "NG_ALLOWED_HOSTS"]) == var.private_hostname
    error_message = "Each Angular SSR service must allow only its configured public or private hostname."
  }
  assert {
    condition     = alltrue([for binding in values(google_cloud_run_v2_service_iam_member.api_invoker) : binding.member != "allUsers" && binding.member != "allAuthenticatedUsers"])
    error_message = "The API must not grant public invocation; only SSR service identities are allowed."
  }
  assert {
    condition     = google_cloud_run_v2_service_iam_member.public_web_invoker[0].member == "allUsers" && google_cloud_run_v2_service.web["public"].iap_enabled == false
    error_message = "The public test entry must be publicly reachable and must not require IAP."
  }
  assert {
    condition     = google_cloud_run_v2_service.web["private"].iap_enabled == true && google_cloud_run_v2_service_iam_member.private_iap_invoker[0].member == google_project_service_identity.iap[0].member
    error_message = "The private run.app entry must retain native Cloud Run IAP and grant run.invoker only to the IAP service agent."
  }
  assert {
    condition     = alltrue([for service in values(google_cloud_run_v2_service.web) : service.template[0].vpc_access[0].egress == "ALL_TRAFFIC"])
    error_message = "SSR must retain Direct VPC egress for private service dependencies and IAP key retrieval."
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
    condition     = google_cloud_run_v2_service.web["private"].iap_enabled == true && google_cloud_run_v2_service.web["private"].ingress == (var.enable_edge ? "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER" : "INGRESS_TRAFFIC_ALL")
    error_message = "The private entry must retain native Cloud Run IAP and use the selected ingress stage."
  }
  assert {
    condition     = google_cloud_run_v2_service_iam_member.private_iap_invoker[0].member == google_project_service_identity.iap[0].member
    error_message = "The private service must grant invoker to the explicitly provisioned IAP service identity."
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

run "candidate_mode_targets_only_the_pinned_gcp06_database" {
  command = plan

  variables {
    project_id                           = "espacigo-testprod"
    project_number                       = "123456789012"
    enable_foundation                    = true
    enable_database                      = true
    enable_services                      = true
    api_contract_encryption_key_version  = "1"
    confirm_project_inventory            = true
    confirm_cost_policy_review           = true
    acknowledge_gcp04_runtime_gates      = true
    bucket_suffix                        = "offline-test"
    api_image                            = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                       = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address              = "10.80.0.10:1025"
    confirm_runtime_secret_versions      = true
    confirm_runtime_database_bootstrap   = true
    api_gcp06_candidate_run_id           = "20261020-gcp06a"
    api_gcp06_candidate_password_version = "3"
  }

  override_data {
    target = data.google_sql_database_instance.gcp06_candidate[0]
    values = {
      connection_name    = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
      database_version   = "POSTGRES_18"
      private_ip_address = "10.189.0.13"
      region             = "southamerica-west1"
    }
  }

  assert {
    condition     = length(google_sql_database_instance.postgres) == 1 && length(data.google_sql_database_instance.gcp06_candidate) == 1
    error_message = "Candidate mode must retain the managed legacy instance while reading the existing GCP-06 target; it must not destroy the old database."
  }
  assert {
    condition     = local.api_sql_connection_name == "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg" && local.api_database_name == "espacigo_gcp06_candidate_20261020-gcp06a"
    error_message = "The API revision must target the pinned candidate instance and database."
  }
  assert {
    condition     = local.web_services["public"].name == "espacigo-test-public" && local.web_services["private"].name == "espacigo-test-private" && length(local.web_services) == 2
    error_message = "Candidate mode must retain the existing public/private frontend services."
  }
  assert {
    condition     = local.api_database_secret_key == "database-password-gcp06" && local.api_database_password_version == "3"
    error_message = "Candidate API must mount the separately versioned candidate DB secret without reading its payload."
  }
  assert {
    condition = (
      contains(local.runtime_secret_ids, "database-password") &&
      contains(local.runtime_secret_ids, "database-password-gcp06") &&
      contains(local.api_secret_access_keys, "database-password") &&
      contains(local.api_secret_access_keys, "database-password-gcp06")
    )
    error_message = "Candidate mode must retain the existing DB secret resource/access for rollback while adding candidate-only access."
  }
}

run "candidate_mode_rejects_a_different_run_id" {
  command = plan

  variables {
    project_id                 = "espacigo-testprod"
    region                     = "southamerica-west1"
    api_gcp06_candidate_run_id = "20261021-gcp06b"
  }

  expect_failures = [var.api_gcp06_candidate_run_id]
}

run "candidate_jobs_are_one_shot_private_and_separately_secret_scoped" {
  command = plan

  variables {
    project_id                             = "espacigo-testprod"
    project_number                         = "123456789012"
    enable_foundation                      = true
    confirm_project_inventory              = true
    confirm_cost_policy_review             = true
    acknowledge_gcp04_runtime_gates        = true
    bucket_suffix                          = "candidate-jobs"
    api_gcp06_candidate_run_id             = "20261020-gcp06a"
    api_gcp06_candidate_password_version   = "3"
    enable_gcp06_admin_bootstrap_job       = true
    enable_gcp06_test_access_job           = true
    enable_gcp06_jobs_reviewed             = true
    gcp06_admin_bootstrap_image            = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp06-admin-bootstrap@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    gcp06_activation_login                 = "gadm_activation_012345abcdef"
    gcp06_admin_bootstrap_password_version = "4"
    gcp06_bootstrap_operator_member        = "user:operator@example.test"
    gcp06_test_access_image                = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp06-test-access@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    gcp06_test_access_login                = "gcp06_test_access_012345abcdef"
    gcp06_test_access_db_password_version  = "5"
    gcp06_test_access_operator_member      = "user:operator@example.test"
  }

  override_data {
    target = data.google_sql_database_instance.gcp06_candidate[0]
    values = {
      connection_name    = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
      database_version   = "POSTGRES_18"
      private_ip_address = "10.189.0.13"
      region             = "southamerica-west1"
    }
  }

  assert {
    condition = (
      length(google_cloud_run_v2_job.gcp06_admin_bootstrap) == 1 &&
      google_cloud_run_v2_job.gcp06_admin_bootstrap[0].template[0].task_count == 1 &&
      google_cloud_run_v2_job.gcp06_admin_bootstrap[0].template[0].template[0].max_retries == 0 &&
      google_service_account.gcp06_job["admin-bootstrap"].account_id == "espacigo-gcp06-admin-bootstrap"
    )
    error_message = "The owner bootstrap must use its dedicated one-shot identity and no automatic retries."
  }
  assert {
    condition = (
      length(google_cloud_run_v2_job.gcp06_test_access) == 1 &&
      google_cloud_run_v2_job.gcp06_test_access[0].template[0].template[0].max_retries == 0 &&
      google_service_account.gcp06_job["test-access"].account_id == "espacigo-gcp06-test-access" &&
      length(google_secret_manager_secret_iam_member.gcp06_job_secret_access) == 3
    )
    error_message = "Preview Jobs must use isolated identities and only the three required SQL secrets."
  }
  assert {
    condition     = one([for env in google_cloud_run_v2_job.gcp06_test_access[0].template[0].template[0].containers[0].env : env.value if env.name == "GCP06_TEST_ACCESS_MODE"]) == "preview"
    error_message = "Test-access Job must default to a non-mutating preview."
  }
}

run "services_without_edge_expose_runapp_with_private_gates" {
  command = plan

  variables {
    project_id                          = "offline-test-project"
    project_number                      = "123456789012"
    enable_foundation                   = true
    enable_database                     = true
    enable_services                     = true
    api_contract_encryption_key_version = "1"
    enable_edge                         = false
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    bucket_suffix                       = "offline-test"
    api_image                           = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                      = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address             = "10.80.0.10:1025"
    confirm_pga_iap_key_preflight       = true
    confirm_runtime_secret_versions     = true
    confirm_runtime_database_bootstrap  = true
    private_demo_origin_override        = "https://private-123.run.app"
  }

  assert {
    condition     = google_cloud_run_v2_service.api[0].ingress == "INGRESS_TRAFFIC_ALL" && alltrue([for service in values(google_cloud_run_v2_service.web) : service.ingress == "INGRESS_TRAFFIC_ALL"])
    error_message = "The first demo endpoints must be network-reachable by run.app."
  }
  assert {
    condition     = length(google_cloud_run_v2_service_iam_member.api_invoker) == 2 && alltrue([for binding in values(google_cloud_run_v2_service_iam_member.api_invoker) : startswith(binding.member, "serviceAccount:") && binding.member != "allUsers" && binding.member != "allAuthenticatedUsers"])
    error_message = "Only the two SSR runtime service accounts may invoke the API."
  }
  assert {
    condition     = google_cloud_run_v2_service_iam_member.public_web_invoker[0].member == "allUsers" && google_cloud_run_v2_service.web["private"].iap_enabled == true && google_cloud_run_v2_service_iam_member.private_iap_invoker[0].member == google_project_service_identity.iap[0].member
    error_message = "Public web stays public while private web requires native IAP and its IAP service agent."
  }
  assert {
    condition     = alltrue([for service in values(google_cloud_run_v2_service.web) : service.template[0].vpc_access[0].egress == "PRIVATE_RANGES_ONLY"])
    error_message = "The run.app-only stage must not claim the edge egress preflight."
  }
  assert {
    condition     = one([for env in google_cloud_run_v2_service.web["public"].template[0].containers[0].env : env.value if env.name == "PRIVATE_DEMO_ORIGIN"]) == var.private_demo_origin_override
    error_message = "The second no-DNS rollout step must route private links to the private IAP-protected run.app origin."
  }
}

run "services_reject_unverified_iap_key_egress" {
  command = plan

  variables {
    project_id                          = "offline-test-project"
    project_number                      = "123456789012"
    enable_foundation                   = true
    enable_database                     = true
    enable_services                     = true
    api_contract_encryption_key_version = "1"
    enable_edge                         = true
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    acknowledge_gcp04_runtime_gates     = true
    bucket_suffix                       = "offline-test"
    api_image                           = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                      = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address             = "10.80.0.10:1025"
    confirm_runtime_secret_versions     = true
    confirm_runtime_database_bootstrap  = true
  }

  expect_failures = [var.enable_edge]
}

run "mailpit_vm_is_private_and_scoped" {
  command = plan

  variables {
    project_id                 = "offline-test-project"
    project_number             = "123456789012"
    enable_foundation          = true
    enable_mailpit             = true
    confirm_project_inventory  = true
    confirm_cost_policy_review = true
    bucket_suffix              = "offline-test"
    iap_evaluator_members      = ["user:tester@example.test"]
  }

  assert {
    condition     = length(google_compute_instance.mailpit[0].network_interface[0].access_config) == 0
    error_message = "The Mailpit VM must not have a public IP address."
  }
  assert {
    condition     = toset(google_compute_firewall.mailpit_smtp[0].source_ranges) == toset([var.api_serverless_subnet_cidr]) && toset(one([for rule in google_compute_firewall.mailpit_smtp[0].allow : rule.ports if rule.protocol == "tcp"])) == toset(["1025"])
    error_message = "SMTP must be limited to the dedicated API subnet on port 1025."
  }
  assert {
    condition     = toset(google_compute_firewall.mailpit_ui_iap[0].source_ranges) == toset(["35.235.240.0/20"]) && toset(one([for rule in google_compute_firewall.mailpit_ui_iap[0].allow : rule.ports if rule.protocol == "tcp"])) == toset(["8025"])
    error_message = "The Mailpit UI must be reachable only through the IAP TCP forwarding source range on port 8025."
  }
  assert {
    condition     = google_iap_tunnel_instance_iam_member.mailpit_ui["user:tester@example.test"].condition[0].expression == "destination.port == 8025"
    error_message = "The explicit evaluator's IAP tunnel grant must be restricted to the Mailpit UI port."
  }
  assert {
    condition     = google_compute_instance.mailpit[0].boot_disk[0].initialize_params[0].image == "projects/cos-cloud/global/images/cos-stable-test" && length(data.google_compute_image.mailpit_cos) == 1
    error_message = "The Mailpit VM must keep the cos-stable family lookup when no image pin is supplied."
  }
  assert {
    condition     = google_artifact_registry_repository.mailpit_dockerhub[0].mode == "REMOTE_REPOSITORY" && google_artifact_registry_repository_iam_member.mailpit_reader[0].role == "roles/artifactregistry.reader"
    error_message = "The VM must pull the pinned upstream image through a read-only Artifact Registry proxy."
  }
  assert {
    condition     = strcontains(google_compute_instance.mailpit[0].metadata_startup_script, "b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48")
    error_message = "The private VM must use the same digest-pinned Mailpit image as Compose."
  }
}

run "mailpit_vm_uses_explicit_cos_image_pin" {
  command = plan

  variables {
    project_id                 = "offline-test-project"
    project_number             = "123456789012"
    enable_foundation          = true
    enable_mailpit             = true
    confirm_project_inventory  = true
    confirm_cost_policy_review = true
    bucket_suffix              = "offline-test"
    mailpit_cos_image          = "https://www.googleapis.com/compute/v1/projects/cos-cloud/global/images/cos-stable-121-18867-638-5"
  }

  assert {
    condition     = google_compute_instance.mailpit[0].boot_disk[0].initialize_params[0].image == var.mailpit_cos_image && length(data.google_compute_image.mailpit_cos) == 0
    error_message = "An explicit COS pin must be used directly without querying the moving cos-stable family."
  }
}

run "services_reject_missing_runtime_secret_versions" {
  command = plan

  variables {
    project_id                          = "offline-test-project"
    project_number                      = "123456789012"
    enable_foundation                   = true
    enable_database                     = true
    enable_services                     = true
    api_contract_encryption_key_version = "1"
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    acknowledge_gcp04_runtime_gates     = true
    confirm_pga_iap_key_preflight       = true
    confirm_runtime_database_bootstrap  = true
    bucket_suffix                       = "offline-test"
    api_image                           = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                      = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address             = "10.80.0.10:1025"
  }

  expect_failures = [var.enable_services]
}

run "services_reject_missing_runtime_database_bootstrap" {
  command = plan

  variables {
    project_id                          = "offline-test-project"
    project_number                      = "123456789012"
    enable_foundation                   = true
    enable_database                     = true
    enable_services                     = true
    api_contract_encryption_key_version = "1"
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    acknowledge_gcp04_runtime_gates     = true
    confirm_pga_iap_key_preflight       = true
    confirm_runtime_secret_versions     = true
    bucket_suffix                       = "offline-test"
    api_image                           = "southamerica-west1-docker.pkg.dev/offline-test/repo/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    frontend_image                      = "southamerica-west1-docker.pkg.dev/offline-test/repo/web@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    mailpit_private_address             = "10.80.0.10:1025"
  }

  expect_failures = [var.enable_services]
}
