variable "name" {
  description = "Name of the IAM role"
  type        = string
}

variable "assume_role_policy" {
  description = "JSON assume-role policy document"
  type        = string
}

variable "basic_execution_policy_arn" {
  description = "ARN of the managed policy to attach (defaults to AWSLambdaBasicExecutionRole)"
  type        = string
  default     = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

variable "sqs_send_arns" {
  description = "List of SQS queue ARNs this role is allowed to send messages to"
  type        = list(string)
  default     = []
}
