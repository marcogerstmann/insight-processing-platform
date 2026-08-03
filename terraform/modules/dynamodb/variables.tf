variable "name" {
  type        = string
  description = "DynamoDB table name"
}

variable "tags" {
  type        = map(string)
  description = "Tags to attach to resources"
  default     = {}
}

variable "enable_tag_gsi" {
  type        = bool
  description = "Add the sparse gsi1 index (gsi1pk/gsi1sk) used for tag membership queries"
  default     = false
}