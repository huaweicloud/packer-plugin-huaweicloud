variable "access_key" {
  type        = string
  default     = env("HW_ACCESS_KEY")
  description = "your access key"
}

variable "secret_key" {
  type        = string
  default     = env("HW_SECRET_KEY")
  sensitive   = true
  description = "your secret key"
}

variable "region" {
  type    = string
}

variable "vpc_id" {
  type    = string
  default = "your vpc id"
}

variable "subnet_id" {
  type    = string
  default = "your subnet id"
}
