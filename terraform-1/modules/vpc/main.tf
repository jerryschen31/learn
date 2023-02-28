# modules/vpc/main.tf
resource "aws_vpc" "example" {
  cidr_block = var.cidr_block
}

resource "aws_internet_gateway" "example" {
  vpc_id = aws_vpc.example.id
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.example.id
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

variable "cidr_block" {
  type    = string
  default = "10.0.0.0/16"
}