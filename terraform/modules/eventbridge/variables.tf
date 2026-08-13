variable "name" {
  description = "Name of the custom EventBridge bus"
  type        = string
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}
