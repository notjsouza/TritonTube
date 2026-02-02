output "frontend_cdn_id" {
  description = "CloudFront distribution ID for frontend"
  value       = module.cloudfront.frontend_cdn_id
}
output "vpc_id" {
  description = "ID of the VPC"
  value       = module.networking.vpc_id
}

output "video_bucket_name" {
  description = "Name of the video content S3 bucket"
  value       = module.s3.video_bucket_id
}

output "frontend_bucket_name" {
  description = "Name of the frontend S3 bucket"
  value       = module.s3.frontend_bucket_id
}

output "uploads_bucket_name" {
  description = "Name of the uploads S3 bucket"
  value       = module.s3.uploads_bucket_id
}

output "video_cdn_domain" {
  description = "CloudFront distribution domain for video content"
  value       = module.cloudfront.video_cdn_domain
}

output "frontend_cdn_domain" {
  description = "CloudFront distribution domain for frontend"
  value       = module.cloudfront.frontend_cdn_domain
}

output "frontend_website_url" {
  description = "S3 website endpoint for frontend"
  value       = module.s3.frontend_bucket_website_endpoint
}

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = module.ecs.alb_dns_name
}

output "api_endpoint" {
  description = "API endpoint URL"
  value       = "http://${module.ecs.alb_dns_name}"
}

output "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecr_repository_url" {
  description = "ECR repository URL for backend image"
  value       = module.ecs.ecr_repository_url
}

output "sqs_upload_queue_url" {
  description = "SQS queue URL for upload processing jobs"
  value       = module.ecs.upload_queue_url
}

output "sqs_upload_queue_arn" {
  description = "SQS queue ARN for upload processing jobs"
  value       = module.ecs.upload_queue_arn
}

output "deployment_summary" {
  description = "Summary of deployed resources"
  value = <<-EOT
    
    ╔════════════════════════════════════════════════════════════════╗
    ║           TritonTube AWS Deployment Summary                    ║
    ╠════════════════════════════════════════════════════════════════╣
    ║                                                                ║
    ║  Frontend URL (CloudFront):                                    ║
    ║    https://${module.cloudfront.frontend_cdn_domain}
    ║                                                                ║
    ║  API Endpoint:                                                 ║
    ║    http://${module.ecs.alb_dns_name}
    ║                                                                ║
    ║  Video CDN:                                                    ║
    ║    https://${module.cloudfront.video_cdn_domain}
    ║                                                                ║
    ║  ECR Repository:                                               ║
    ║    ${module.ecs.ecr_repository_url}
    ║                                                                ║
    ║  Next Steps:                                                   ║
    ║    1. Build and push Docker image to ECR                       ║
    ║    2. Update container_image variable with ECR URL             ║
    ║    3. Run: terraform apply -var="container_image=<ecr-url>"    ║
    ║    4. Build and deploy frontend to S3                          ║
    ║                                                                ║
    ╚════════════════════════════════════════════════════════════════╝
  EOT
}
