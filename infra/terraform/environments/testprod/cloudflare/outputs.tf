output "managed_hostnames" {
  value       = var.manage_dns_records ? [cloudflare_dns_record.public_entry[0].name, cloudflare_dns_record.private_entry[0].name] : []
  description = "Only the two approved EspaciGo DNS record names; no zone settings or unrelated records are managed."
}
