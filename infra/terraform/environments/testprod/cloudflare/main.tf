terraform {
  required_version = ">= 1.16.0, < 1.17.0"

  backend "gcs" {}

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.24.0"
    }
  }
}

provider "cloudflare" {}

resource "cloudflare_dns_record" "public_entry" {
  count = var.manage_dns_records ? 1 : 0

  zone_id = var.zone_id
  name    = var.public_hostname
  type    = "A"
  content = var.load_balancer_ipv4
  ttl     = 1
  proxied = false
  comment = "EspaciGo testprod public HTTPS entry; Terraform-managed DNS-only record."
}

resource "cloudflare_dns_record" "private_entry" {
  count = var.manage_dns_records ? 1 : 0

  zone_id = var.zone_id
  name    = var.private_hostname
  type    = "A"
  content = var.load_balancer_ipv4
  ttl     = 1
  proxied = false
  comment = "EspaciGo testprod private HTTPS entry; IAP is enforced by Cloud Run."
}
