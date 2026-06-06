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