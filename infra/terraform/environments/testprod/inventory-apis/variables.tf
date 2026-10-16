variable "project_id" {
  type        = string
  description = "Existing project where only the two inventory APIs may be enabled."
  default     = "espacigo-testprod"
  validation {
    condition     = var.project_id == "espacigo-testprod"
    error_message = "This isolated inventory step is scoped only to espacigo-testprod."
  }
}

variable "services_to_enable" {
  type        = list(string)
  description = "Explicitly list only inventory APIs verified disabled immediately before the plan."
  default     = []
  validation {
    condition = (
      length(toset(var.services_to_enable)) == length(var.services_to_enable) &&
      alltrue([for service in var.services_to_enable : contains(["orgpolicy.googleapis.com", "cloudasset.googleapis.com"], service)])
    )
    error_message = "Only unique orgpolicy.googleapis.com and cloudasset.googleapis.com entries are allowed."
  }
}
