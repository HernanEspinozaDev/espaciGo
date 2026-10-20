mock_provider "google" {
  override_during = plan

  mock_data "google_compute_network" {
    defaults = {
      self_link = "projects/espacigo-testprod/global/networks/espacigo-testprod"
      name      = "espacigo-testprod"
    }
  }

  mock_resource "google_sql_database_instance" {
    defaults = {
      service_account_email_address = "p447373903153-abc@gcp-sa-cloud-sql.iam.gserviceaccount.com"
    }
  }

  mock_resource "google_project_iam_custom_role" {
    defaults = {
      name = "projects/espacigo-testprod/roles/gcp06CandidateObjectGetreview01"
    }
  }
}

run "default_off_is_noop_and_destroy_is_denied" {
  command = plan

  assert {
    condition = (
      length(data.google_compute_network.candidate) == 0 &&
      length(google_sql_database_instance.candidate) == 0 &&
      length(google_sql_database.candidate) == 0 &&
      length(google_storage_bucket.import) == 0 &&
      length(google_project_iam_custom_role.import_object_reader) == 0 &&
      length(google_storage_bucket_iam_member.cloud_sql_import_reader) == 0 &&
      length(google_project_iam_custom_role.upload_object_creator) == 0 &&
      length(google_storage_bucket_iam_member.uploader) == 0 &&
      local.protect_candidate
    )
    error_message = "GCP-06 must plan no Cloud access or resources by default and deny destroy."
  }

  assert {
    condition = (
      var.project_id == "espacigo-testprod" &&
      var.region == "southamerica-west1" &&
      var.network_name == "espacigo-testprod"
    )
    error_message = "GCP-06 target project, region and network must remain pinned."
  }
}

run "candidate_and_exact_temporary_bindings_are_scoped" {
  command = plan

  variables {
    run_id                           = "review01"
    enable_candidate                 = true
    confirm_candidate_release        = true
    enable_import_binding            = true
    confirm_import_binding           = true
    import_binding_expires_at        = formatdate("YYYY-MM-DD'T'hh:mm:ssZ", timeadd(timestamp(), "24h"))
    enable_upload_binding            = true
    confirm_upload_binding           = true
    uploader_member                  = "user:reviewer@example.invalid"
    upload_binding_expires_at        = formatdate("YYYY-MM-DD'T'hh:mm:ssZ", timeadd(timestamp(), "24h"))
    allow_candidate_destroy          = true
    confirm_candidate_destroy_run_id = "wrong-run"
  }

  assert {
    condition = (
      length(google_sql_database_instance.candidate) == 1 &&
      google_sql_database_instance.candidate[0].database_version == "POSTGRES_18" &&
      google_sql_database_instance.candidate[0].region == "southamerica-west1" &&
      google_sql_database_instance.candidate[0].deletion_protection &&
      google_sql_database_instance.candidate[0].settings[0].ip_configuration[0].ipv4_enabled == false &&
      local.protect_candidate
    )
    error_message = "Candidate must be PG18, private, region-pinned, and protected unless exact destroy confirmation matches."
  }

  assert {
    condition = (
      google_storage_bucket.import[0].uniform_bucket_level_access &&
      google_storage_bucket.import[0].public_access_prevention == "enforced" &&
      one([for condition in google_storage_bucket.import[0].lifecycle_rule[0].condition : condition.age]) == 7
    )
    error_message = "The import bucket must use UBLA, prevent public access and expire the object after seven days."
  }

  assert {
    condition     = google_project_iam_custom_role.import_object_reader[0].permissions == toset(["storage.objects.get"])
    error_message = "Cloud SQL import role must grant only object get."
  }

  assert {
    condition     = google_storage_bucket_iam_member.cloud_sql_import_reader[0].condition[0].expression == "resource.name == 'projects/_/buckets/espacigo-testprod-gcp06-review01/objects/gcp06/review01/data.sql' && request.time < timestamp('${var.import_binding_expires_at}')"
    error_message = "Cloud SQL import binding must be restricted to the exact object and expire."
  }

  assert {
    condition     = google_project_iam_custom_role.upload_object_creator[0].permissions == toset(["storage.objects.create"])
    error_message = "Uploader role must grant only object create."
  }

  assert {
    condition     = google_storage_bucket_iam_member.uploader[0].member == "user:reviewer@example.invalid" && google_storage_bucket_iam_member.uploader[0].condition[0].expression == "resource.name == 'projects/_/buckets/espacigo-testprod-gcp06-review01/objects/gcp06/review01/data.sql' && request.time < timestamp('${var.upload_binding_expires_at}')"
    error_message = "Uploader binding must be restricted to the exact object and expire."
  }
}
