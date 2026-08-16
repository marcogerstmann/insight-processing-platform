variable "bus_name" {
  description = "Name of the EventBridge bus to subscribe to"
  type        = string
}

variable "subscriber_name" {
  description = "Name of the subscriber; used to name the rule, queue, and DLQ"
  type        = string
}

variable "detail_types" {
  description = "Domain event types (EventBridge detail-type) this subscriber matches"
  type        = list(string)
}

variable "max_receive_count" {
  description = "How often a message can be received before moving to the DLQ"
  type        = number
  default     = 5
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}
