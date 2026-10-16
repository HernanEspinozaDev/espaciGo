output "state_bucket_name" {
  value       = { for key, bucket in google_storage_bucket.terraform_state : key => bucket.name }
  description = "Copy each result privately into the matching root's backend config after bootstrap review."
}

output "state_bucket_urls" {
  value       = { for key, bucket in google_storage_bucket.terraform_state : key => bucket.url }
  description = "Private bucket endpoints; do not expose or copy state contents."
}
