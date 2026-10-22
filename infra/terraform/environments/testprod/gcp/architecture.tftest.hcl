mock_provider "google" {
  override_during = plan

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

  mock_data "google_sql_database" {
    defaults = {
      name = "espacigo_dms_stage_20261020"
    }
  }

  mock_data "google_secret_manager_secret" {
    defaults = {
      secret_id = "espacigo-test-database-password-v67"
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
    condition     = length(google_project_service.required) == 15
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
  assert {
    condition     = google_iam_workload_identity_pool_provider.github[0].attribute_mapping["attribute.repository_id"] == "assertion.repository_id" && google_iam_workload_identity_pool_provider.github[0].attribute_mapping["attribute.repository_owner_id"] == "assertion.repository_owner_id" && strcontains(google_iam_workload_identity_pool_provider.github[0].attribute_condition, "refs/heads/main") && strcontains(google_iam_workload_identity_pool_provider.github[0].attribute_condition, "environment")
    error_message = "Dormant legacy GitHub federation must remain limited to the immutable repository identity, main branch, and protected environment until separate IAM cleanup."
  }
  assert {
    condition     = google_storage_managed_folder.terraform_state[0].name == "espacigo/testprod/gcp/" && google_storage_managed_folder_iam_member.deployer_terraform_state[0].managed_folder == "espacigo/testprod/gcp/"
    error_message = "The deployer state grant must be scoped to the managed folder for the canonical Terraform state and lock prefix."
  }
  assert {
    condition = (
      !var.enable_database &&
      !var.enable_gcp06_jobs_reviewed &&
      contains(local.runtime_secret_ids, "database-migrator-password") &&
      contains(local.runtime_secret_ids, "gcp06-sql-inspector-credential") &&
      google_secret_manager_secret.runtime["database-migrator-password"].secret_id == "espacigo-test-database-migrator-password" &&
      google_secret_manager_secret.runtime["gcp06-sql-inspector-credential"].secret_id == "espacigo-test-gcp06-sql-inspector-credential" &&
      length(google_secret_manager_secret_iam_member.migrator_db_access) == 1 &&
      google_secret_manager_secret_iam_member.migrator_db_access[0].secret_id == "espacigo-test-database-migrator-password"
    )
    error_message = "The existing migrator grant and database-migrator/SQL-inspector secret containers must remain managed while the optional gates are off."
  }
  assert {
    condition = (
      length(google_service_account.cloud_build_deployer) == 1 &&
      google_service_account.cloud_build_deployer[0].account_id == "espacigo-cloud-build-deployer" &&
      length(google_project_iam_member.cloud_build_deployer) == 1 &&
      google_project_iam_custom_role.cloud_build_deployer[0].role_id == "espacigoCloudBuildDeployer" &&
      contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "containeranalysis.occurrences.list") &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "containeranalysis.occurrences.create") &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "containeranalysis.occurrences.delete") &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "containeranalysis.occurrences.update") &&
      google_artifact_registry_repository_iam_member.cloud_build_writer[0].role == "roles/artifactregistry.writer" &&
      google_project_iam_member.cloud_build_log_writer[0].role == "roles/logging.logWriter" &&
      length(google_service_account_iam_member.cloud_build_runtime_act_as) == 3 &&
      google_service_account_iam_member.cloud_build_self_act_as[0].role == "roles/iam.serviceAccountUser" &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "run.services.delete") &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "run.jobs.run") &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "secretmanager.versions.access") &&
      !contains(google_project_iam_custom_role.cloud_build_deployer[0].permissions, "cloudsql.instances.connect")
    )
    error_message = "The Cloud Build identity must be dedicated and minimally scoped without SQL, secret reads, deletes, or Jobs."
  }
  assert {
    condition = (
      google_storage_bucket.cloud_build[0].name == "espacigo-testprod-cloud-build-123456789012" &&
      google_storage_bucket.cloud_build[0].uniform_bucket_level_access &&
      google_storage_bucket.cloud_build[0].public_access_prevention == "enforced" &&
      google_storage_bucket.cloud_build[0].force_destroy == false &&
      one(one(google_storage_bucket.cloud_build[0].lifecycle_rule).condition).age == 30 &&
      contains(one(one(google_storage_bucket.cloud_build[0].lifecycle_rule).condition).matches_prefix, "source/") &&
      google_storage_bucket_iam_member.cloud_build_bucket_operator[0].bucket == google_storage_bucket.cloud_build[0].name &&
      google_project_iam_custom_role.cloud_build_bucket_operator[0].role_id == "espacigoCloudBuildBucketOperator" &&
      length(google_project_iam_custom_role.cloud_build_bucket_operator[0].permissions) == 5
    )
    error_message = "The dedicated Cloud Build bucket must be private, project-unique, source-expiring, protected from force deletion, and use its narrow bucket-scoped role."
  }
}

run "edge_targets_external_cloud_run_service_names" {
  command = plan

  variables {
    project_id                          = "offline-test-project"
    project_number                      = "123456789012"
    enable_foundation                   = true
    enable_edge                         = true
    confirm_external_cloud_run_services = true
    confirm_pga_iap_key_preflight       = true
    confirm_project_inventory           = true
    confirm_cost_policy_review          = true
    bucket_suffix                       = "offline-test"
  }

  assert {
    condition = (
      google_compute_region_network_endpoint_group.web["public"].cloud_run[0].service == "espacigo-test-public" &&
      google_compute_region_network_endpoint_group.web["private"].cloud_run[0].service == "espacigo-test-private" &&
      length(google_compute_url_map.edge) == 1
    )
    error_message = "The Terraform-owned HTTPS edge must target the externally managed service names after explicit inventory confirmation."
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
