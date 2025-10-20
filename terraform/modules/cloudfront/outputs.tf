output "video_cdn_domain" {
  description = "CloudFront domain for video content"
  value       = aws_cloudfront_distribution.video_content.domain_name
}

output "video_cdn_id" {
  description = "CloudFront distribution ID for video content"
  value       = aws_cloudfront_distribution.video_content.id
}

output "frontend_cdn_domain" {
  description = "CloudFront domain for frontend"
  value       = aws_cloudfront_distribution.frontend.domain_name
}

output "frontend_cdn_id" {
  description = "CloudFront distribution ID for frontend"
  value       = aws_cloudfront_distribution.frontend.id
}
