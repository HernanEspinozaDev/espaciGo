output "candidate_instance_name" {
  value = one(google_sql_database_instance.candidate[*].name)
}

output "candidate_database_name" {
  value = one(google_sql_database.candidate[*].name)
}

output "private_import_bucket" {
  value = one(google_storage_bucket.import[*].name)
}
