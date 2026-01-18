variable "name" {
  type = string
}

variable "assume_role_policy" {
  type = string
}

variable "basic_execution_policy_arn" {
  type    = string
  default = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

variable "sqs_send_arns" {
  type        = list(string)
  description = "List of SQS queue ARNs this role is allowed to send messages to."
  default     = []
}
