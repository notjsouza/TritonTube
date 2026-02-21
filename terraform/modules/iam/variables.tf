variable "project_name" {
  description = "Project name for resource naming"
  type        = string
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "dynamodb_table_arn" {
  description = "ARN of the DynamoDB table for video metadata"
  type        = string
}

variable "sqs_queue_arn" {
  description = "ARN of the SQS queue for upload jobs"
  type        = string
}

variable "sqs_dlq_arn" {
  description = "ARN of the dead-letter queue for failed upload jobs"
  type        = string
}

variable "s3_bucket_name" {
  description = "Name of the S3 bucket for video content"
  type        = string
}

variable "uploads_bucket_name" {
  description = "Name of the S3 bucket for uploads"
  type        = string
}
