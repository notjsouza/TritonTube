variable "project_name" {
  description = "Project name for resource naming"
  type        = string
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "public_subnet_ids" {
  description = "List of public subnet IDs for ALB"
  type        = list(string)
}

variable "private_subnet_ids" {
  description = "List of private subnet IDs for ECS tasks"
  type        = list(string)
}

variable "task_execution_role_arn" {
  description = "ARN of the ECS task execution role"
  type        = string
}

variable "task_role_arn" {
  description = "ARN of the ECS task role"
  type        = string
}

variable "container_image" {
  description = "Docker image for the container"
  type        = string
}

variable "container_cpu" {
  description = "CPU units for the container"
  type        = number
  default     = 512
}

variable "container_memory" {
  description = "Memory for the container in MB"
  type        = number
  default     = 1024
}

variable "desired_count" {
  description = "Desired number of ECS tasks"
  type        = number
  default     = 2
}

variable "s3_bucket" {
  description = "S3 bucket name for video storage"
  type        = string
}

variable "uploads_bucket" {
  description = "S3 bucket name for uploads (temporary)"
  type        = string
}

variable "worker_image" {
  description = "Docker image for the worker container"
  type        = string
  default     = ""
}

variable "worker_cpu" {
  description = "CPU units for worker task"
  type        = number
  default     = 512
}

variable "worker_memory" {
  description = "Memory (MB) for worker task"
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

variable "cdn_domain" {
  description = "CloudFront CDN domain"
  type        = string
}
