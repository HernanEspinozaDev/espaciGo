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
}
