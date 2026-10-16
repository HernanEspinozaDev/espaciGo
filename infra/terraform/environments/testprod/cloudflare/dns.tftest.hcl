mock_provider "cloudflare" {
  override_during = plan
}

run "only_two_dns_only_records_are_managed" {
  command = plan

  variables {
    manage_dns_records = true
    zone_id            = "offline-zone-id"
    load_balancer_ipv4 = "203.0.113.10"
  }

  assert {
    condition     = cloudflare_dns_record.public_entry[0].name == "espacigo.testingpage.store" && cloudflare_dns_record.public_entry[0].proxied == false && cloudflare_dns_record.public_entry[0].content == "203.0.113.10"
    error_message = "The public hostname must be one DNS-only A record to the reviewed load balancer IP."
  }
  assert {
    condition     = cloudflare_dns_record.private_entry[0].name == "espacigo-demo.testingpage.store" && cloudflare_dns_record.private_entry[0].proxied == false && cloudflare_dns_record.private_entry[0].content == "203.0.113.10"
    error_message = "The private hostname must be one DNS-only A record to the reviewed load balancer IP."
  }
}
