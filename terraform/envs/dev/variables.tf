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

variable "domain_name" {
  description = "Custom domain for the web app, fronted by CloudFront"
  type        = string
  default     = "ipp.marcogerstmann.com"
}

variable "api_domain_name" {
  description = "Custom domain for the REST API"
  type        = string
  default     = "api.ipp.marcogerstmann.com"
}

variable "raindrop_poll_interval_hours" {
  description = "How often the Raindrop poll Lambda runs. A cost/noise knob, not a capacity one — the free tier and Raindrop's 120 req/min rate limit are nowhere near binding."
  type        = number
  default     = 3
}

variable "raindrop_poll_limit" {
  description = "Max highlights the Raindrop poll enqueues per run"
  type        = number
  default     = 50
}
