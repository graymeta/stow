variable "resource_group_name" {
  type        = string
  description = "The associated resource group id for module resources"
}

variable "location" {
  type        = string
  description = "The location for the resources (defaults to the RG resource group)"
  default     = null
}

variable "tags" {
  type        = map(string)
  description = "Common tags to be added to all resources which can take tags"
  default     = {}
}