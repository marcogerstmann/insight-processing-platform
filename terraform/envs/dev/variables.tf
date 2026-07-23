variable "region" {
  description = "AWS region to deploy resources into"
  type        = string
  default     = "eu-central-1"
}

variable "project" {
  description = "Project name prefix"
  type        = string
  default     = "ipp"
}

variable "env" {
  description = "Deployment environment"
  type        = string
  default     = "dev"
}

variable "worker_image_uri" {
  description = "Full ECR image URI for the worker Lambda"
  type        = string
}

variable "default_tenant_id" {
  description = "Defautlt tenant ID"
  type        = string
  default     = "test-tenant-id"
}

variable "web_app_origins" {
  description = "Browser origins allowed to call the REST API (CORS). Includes the Vite dev server; append the deployed web app origin here."
  type        = list(string)
  default     = ["http://localhost:5173"]
}
