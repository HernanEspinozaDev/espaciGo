variable "enable_profile_space_media_bucket" {
  type        = bool
  description = "Create only the private profile/space media bucket and its API-scoped object grant."
  default     = false

  validation {
    condition = !var.enable_profile_space_media_bucket || (
      var.project_id == "espacigo-testprod" &&
      var.project_number == "447373903153" &&
      var.region == "southamerica-west1" &&
      var.bucket_suffix == "447373903153" &&
      var.confirm_project_inventory &&
      var.confirm_cost_policy_review
    )
    error_message = "The media-only bucket gate requires the reviewed testprod project/region, valid suffix, and explicit inventory/cost confirmations."
  }
}
