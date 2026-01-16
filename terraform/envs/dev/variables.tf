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
