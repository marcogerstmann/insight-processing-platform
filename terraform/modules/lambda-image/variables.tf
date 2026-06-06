variable "name" {
  description = "Lambda function name"
  type        = string
}

variable "role_arn" {
  description = "ARN of the IAM execution role"
  type        = string
}

variable "image_uri" {
  description = "Full ECR image URI (including tag or digest) to deploy"
  type        = string
}

variable "timeout" {
  description = "Maximum execution time in seconds"
  type        = number
  default     = 10
}

variable "memory_size" {
  description = "Amount of memory in MB allocated to the function"
  type        = number
  default     = 128
}

variable "environment_variables" {
  description = "Environment variables passed to the Lambda function"
  type        = map(string)
  default     = {}
}
