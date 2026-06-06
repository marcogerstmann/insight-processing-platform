variable "name" {
  description = "Lambda function name"
  type        = string
}

variable "role_arn" {
  description = "ARN of the IAM execution role"
  type        = string
}

variable "filename" {
  description = "Path to the ZIP deployment package"
  type        = string
}

variable "source_code_hash" {
  description = "Base64-encoded SHA256 of the deployment package (triggers updates)"
  type        = string
}

variable "handler" {
  description = "Function entrypoint (e.g. \"bootstrap\" for provided.al2023)"
  type        = string
}

variable "runtime" {
  description = "Lambda runtime identifier"
  type        = string
  default     = "provided.al2023"
}

variable "memory_size" {
  description = "Amount of memory in MB allocated to the function"
  type        = number
  default     = 128
}

variable "timeout" {
  description = "Maximum execution time in seconds"
  type        = number
  default     = 5
}

variable "environment_variables" {
  description = "Environment variables passed to the Lambda function"
  type        = map(string)
  default     = {}
}
