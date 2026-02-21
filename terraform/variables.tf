variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-west-1"
}

variable "project_name" {
  description = "Project name used for resource naming"
  type        = string
  default     = "tritontube"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
}

variable "vpc_cidr" {
  description = "CIDR block for VPC"
  type        = string
  default     = "10.0.0.0/16"
}

# ECS Variables
variable "container_image" {
  description = "Docker image for ECS task (update after pushing to ECR)"
  type        = string
  default     = "nginx:latest" # Placeholder - update with your ECR image
}

variable "container_cpu" {
  description = "CPU units for container (1024 = 1 vCPU)"
  type        = number
  default     = 512
}

variable "container_memory" {
  description = "Memory for container in MB"
  type        = number
  default     = 1024
}

variable "desired_count" {
  description = "Desired number of ECS tasks"
  type        = number
  default     = 2
}

variable "worker_image" {
  description = "Docker image for the worker task (defaults to same repository with -worker tag)"
  type        = string
  default     = ""
}

variable "worker_cpu" {
  description = "CPU for worker task"
  type        = number
  default     = 512
}

variable "worker_memory" {
  description = "Memory for worker task in MB"
  type        = number
  default     = 1024
}

variable "worker_desired_count" {
  description = "Initial desired number of worker tasks"
  type        = number
  default     = 0
}

variable "worker_min_count" {
  description = "Minimum number of worker tasks (0 = scale to zero when queue is empty)"
  type        = number
  default     = 0
}

variable "worker_max_count" {
  description = "Maximum number of worker tasks"
  type        = number
  default     = 5
}

variable "uploads_bucket" {
  description = "Uploads bucket (temporary) name"
  type        = string
  default     = ""
}

variable "enable_autoscaling" {
  description = "Enable ECS auto scaling"
  type        = bool
  default     = true
}

variable "min_capacity" {
  description = "Minimum number of ECS tasks"
  type        = number
  default     = 2
}

variable "max_capacity" {
  description = "Maximum number of ECS tasks"
  type        = number
  default     = 10
}
