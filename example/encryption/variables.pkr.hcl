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
  default = "ap-southeast-1"
}

variable "kms_key_id" {
  type = string
}
