variable "name" {
  description = "Name of the HTTP API"
  type        = string
}

variable "lambda_invoke_arn" {
  description = "Invoke ARN of the Lambda function to proxy requests to"
  type        = string
}

variable "route_key" {
  description = "API Gateway route key, e.g. \"POST /readwise/webhook\""
  type        = string
  default     = "POST /readwise/webhook"
}
