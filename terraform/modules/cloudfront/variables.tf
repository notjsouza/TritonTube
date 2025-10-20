variable "project_name" {
  description = "Project name for resource naming"
  type        = string
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "video_bucket_domain_name" {
  description = "Regional domain name of the video content bucket"
  type        = string
}

variable "video_bucket_id" {
  description = "ID of the video content bucket"
  type        = string
}

variable "frontend_bucket_website" {
  description = "S3 website endpoint for frontend bucket"
  type        = string
}
