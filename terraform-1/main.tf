# main.tf
module "vpc" {
  source = "./modules/vpc"
}

module "public_subnet" {
  source  = "./modules/public_subnet"
  vpc_id  = module.vpc.aws_vpc.example.id
}

module "private_subnet" {
  source  = "./modules/private_subnet"
  vpc_id  = module.vpc.aws_vpc.example.id
}

module "ec2" {
  source    = "./modules/ec2"
  subnet_id = module.private_subnet.aws_subnet.private.id
}