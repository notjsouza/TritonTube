terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Optional: Configure backend for state management
  # backend "s3" {
  #   bucket         = "tritontube-terraform-state"
  #   key            = "prod/terraform.tfstate"
  #   region         = "us-west-1"
  #   encrypt        = true
  #   dynamodb_table = "tritontube-terraform-locks"
  # }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = var.project_name
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

# Data sources
data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" {
  state = "available"
}

# IAM Module
module "iam" {
  source = "./modules/iam"

  project_name = var.project_name
  environment  = var.environment
}

# Networking Module
module "networking" {
  source = "./modules/networking"

  project_name        = var.project_name
  environment         = var.environment
  vpc_cidr            = var.vpc_cidr
  availability_zones  = slice(data.aws_availability_zones.available.names, 0, 2)
}

# S3 Module
module "s3" {
  source = "./modules/s3"

  project_name = var.project_name
  environment  = var.environment
}

# CloudFront Module
module "cloudfront" {
  source = "./modules/cloudfront"

  project_name              = var.project_name
  environment               = var.environment
  video_bucket_domain_name  = module.s3.video_bucket_regional_domain_name
  video_bucket_id           = module.s3.video_bucket_id
  frontend_bucket_website   = module.s3.frontend_bucket_website_endpoint
}

# RDS Module
module "rds" {
  source = "./modules/rds"

  project_name           = var.project_name
  environment            = var.environment
  vpc_id                 = module.networking.vpc_id
  database_subnet_ids    = module.networking.database_subnet_ids
  db_subnet_group_name   = module.networking.db_subnet_group_name
  ecs_security_group_id  = module.ecs.ecs_task_security_group_id
  db_username            = var.db_username
  db_password            = var.db_password
  db_instance_class      = var.db_instance_class
}

# ECS Module
module "ecs" {
  source = "./modules/ecs"

  project_name               = var.project_name
  environment                = var.environment
  vpc_id                     = module.networking.vpc_id
  private_subnet_ids         = module.networking.private_subnet_ids
  public_subnet_ids          = module.networking.public_subnet_ids
  task_execution_role_arn    = module.iam.ecs_task_execution_role_arn
  task_role_arn              = module.iam.ecs_task_role_arn
  
  # Application configuration
  container_image            = var.container_image
  container_cpu              = var.container_cpu
  container_memory           = var.container_memory
  desired_count              = var.desired_count
  
  # Environment variables for container
  db_connection_string       = "postgres://${var.db_username}:${var.db_password}@${module.rds.db_endpoint}/${var.db_name}"
  s3_bucket                  = module.s3.video_bucket_id
  cdn_domain                 = module.cloudfront.video_cdn_domain
}
