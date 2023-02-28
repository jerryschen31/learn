# modules/public_subnet/main.tf
resource "aws_subnet" "public" {
  vpc_id     = var.vpc_id
  cidr_block = var.cidr_block
}

variable "vpc_id" {
  type    = string
  default = ""
}

variable "cidr_block" {
  type    = string
  default = "10.0.1.0/24"
}