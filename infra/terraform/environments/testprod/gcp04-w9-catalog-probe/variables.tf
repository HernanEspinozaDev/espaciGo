variable "enable_w9_release" {
  type    = bool
  default = false
}
variable "confirm_w9_release" {
  type    = bool
  default = false
}
variable "project_id" {
  type    = string
  default = "espacigo-testprod"
}
variable "region" {
  type    = string
  default = "southamerica-west1"
}
variable "sql_instance_name" {
  type    = string
  default = "espacigo-testprod-pg"
}
variable "network_name" {
  type    = string
  default = "espacigo-testprod"
}
variable "subnetwork_name" {
  type    = string
  default = "espacigo-testprod-serverless"
}
variable "repository_id" {
  type    = string
  default = "espacigo-testprod"
}
variable "image_digest" {
  type    = string
  default = ""
}
variable "secret_version" {
  type    = string
  default = ""
}
variable "release_id" {
  type    = string
  default = ""
}
variable "release_sha256" {
  type    = string
  default = ""
}
variable "reviewer" {
  type    = string
  default = ""
}
