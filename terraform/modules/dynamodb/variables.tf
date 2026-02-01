variable "name" {
  type        = string
  description = "DynamoDB table name"
}

variable "tags" {
  type        = map(string)
  description = "Tags to attach to resources"
  default     = {}
}