output "video_bucket_id" {
  description = "ID of the video content bucket"
  value       = aws_s3_bucket.video_content.id
}

output "video_bucket_name" {
  description = "Name of the video content bucket"
  value       = aws_s3_bucket.video_content.bucket
}

output "video_bucket_arn" {
  description = "ARN of the video content bucket"
  value       = aws_s3_bucket.video_content.arn
}

output "video_bucket_regional_domain_name" {
  description = "Regional domain name of the video content bucket"
  value       = aws_s3_bucket.video_content.bucket_regional_domain_name
}

output "frontend_bucket_id" {
  description = "ID of the frontend bucket"
  value       = aws_s3_bucket.frontend.id
}

output "frontend_bucket_arn" {
  description = "ARN of the frontend bucket"
  value       = aws_s3_bucket.frontend.arn
}

output "frontend_bucket_website_endpoint" {
  description = "Website endpoint of the frontend bucket"
  value       = aws_s3_bucket_website_configuration.frontend.website_endpoint
}

output "uploads_bucket_id" {
  description = "ID of the uploads bucket"
  value       = aws_s3_bucket.uploads.id
}

output "uploads_bucket_name" {
  description = "Name of the uploads bucket"
  value       = aws_s3_bucket.uploads.bucket
}

output "uploads_bucket_arn" {
  description = "ARN of the uploads bucket"
  value       = aws_s3_bucket.uploads.arn
}
