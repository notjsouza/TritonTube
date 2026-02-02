output "cluster_id" {
  description = "ID of the ECS cluster"
  value       = aws_ecs_cluster.main.id
}

output "cluster_name" {
  description = "Name of the ECS cluster"
  value       = aws_ecs_cluster.main.name
}

output "service_name" {
  description = "Name of the ECS service"
  value       = aws_ecs_service.main.name
}

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = aws_lb.main.dns_name
}

output "alb_arn" {
  description = "ARN of the Application Load Balancer"
  value       = aws_lb.main.arn
}

output "ecs_task_security_group_id" {
  description = "Security group ID for ECS tasks"
  value       = aws_security_group.ecs_tasks.id
}

output "ecr_repository_url" {
  description = "URL of the ECR repository"
  value       = aws_ecr_repository.backend.repository_url
}

output "log_group_name" {
  description = "Name of the CloudWatch log group"
  value       = aws_cloudwatch_log_group.ecs.name
}

output "upload_queue_url" {
  description = "SQS queue URL for upload jobs"
  value       = aws_sqs_queue.upload_jobs.id
}

output "upload_queue_arn" {
  description = "SQS queue ARN for upload jobs"
  value       = aws_sqs_queue.upload_jobs.arn
}

output "dynamodb_table_name" {
  description = "Name of the DynamoDB table for video metadata"
  value       = aws_dynamodb_table.video_metadata.name
}

output "dynamodb_table_arn" {
  description = "ARN of the DynamoDB table for video metadata"
  value       = aws_dynamodb_table.video_metadata.arn
}

output "worker_service_name" {
  description = "Name of the ECS worker service"
  value       = aws_ecs_service.worker.name
}
