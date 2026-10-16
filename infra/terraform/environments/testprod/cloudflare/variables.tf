variable "zone_id" {
  type        = string
  description = "Existing testingpage.store zone ID supplied privately after inventory."
  default     = ""
}

variable "public_hostname" {
  type        = string
  description = "Chosen public entry hostname."
  default     = "espacigo.testingpage.store"
}

variable "private_hostname" {
  type        = string
  description = "Chosen evaluator entry hostname, separately IAP-protected at Cloud Run."
  default     = "espacigo-demo.testingpage.store"
}

variable "load_balancer_ipv4" {
  type        = string
  description = "Static IPv4 output copied from the reviewed GCP plan; never read via shared state."
  default     = ""
}

variable "manage_dns_records" {
  type        = bool
  description = "Explicit DNS mutation gate. Keep false for offline validation."
  default     = false
  validation {
    condition     = !var.manage_dns_records || (var.confirm_dns_inventory && var.zone_id != "" && can(cidrnetmask("${var.load_balancer_ipv4}/32")))
    error_message = "Managing DNS requires explicit zone inventory confirmation, a zone ID, and a valid IPv4 address from the reviewed GCP plan."
  }
}

variable "confirm_dns_inventory" {
  type        = bool
  description = "Set only after reviewing ownership and existing records in the exact Cloudflare zone."
  default     = false
}
