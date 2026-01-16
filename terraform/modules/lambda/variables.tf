variable "name" {
  type = string
}

variable "role_arn" {
  type = string
}

variable "filename" {
  type = string
}

variable "source_code_hash" {
  type = string
}

variable "handler" {
  type = string
}

variable "runtime" {
  type = string
  default = "provided.al2"
}

variable "memory_size" {
  type = number
  default = 128
}

variable "timeout" {
  type = number
  default = 5
}
