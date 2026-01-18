variable "name" {
  description = "Base name for the SQS queue (DLQ will be name + -dlq)"
  type        = string
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}

variable "message_retention_seconds" {
  description = "How long messages are retained in the main queue"
  type        = number
  default     = 345600 # 4 days
}

variable "visibility_timeout_seconds" {
  description = "Visibility timeout for processing messages"
  type        = number
  default     = 60
}

variable "max_receive_count" {
  description = "How often a message can be received before moving to DLQ"
  type        = number
  default     = 5
}

variable "dlq_message_retention_seconds" {
  description = "How long messages are retained in the DLQ"
  type        = number
  default     = 1209600 # 14 days
}

variable "sqs_managed_sse_enabled" {
  description = "Enable SQS-managed server-side encryption"
  type        = bool
  default     = true
}
