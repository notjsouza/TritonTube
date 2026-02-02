# CloudFront Origin Access Control for Video Content
resource "aws_cloudfront_origin_access_control" "video_content" {
  name                              = "${var.project_name}-video-oac"
  description                       = "OAC for video content bucket"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

# CloudFront Distribution for Video Content
resource "aws_cloudfront_distribution" "video_content" {
  enabled             = true
  comment             = "${var.project_name} Video Content CDN"
  default_root_object = ""
  price_class         = "PriceClass_100" # US, Canada, Europe

  origin {
    domain_name              = var.video_bucket_domain_name
    origin_id                = "S3-${var.video_bucket_id}"
    origin_access_control_id = aws_cloudfront_origin_access_control.video_content.id
  }

  default_cache_behavior {
    allowed_methods  = ["GET", "HEAD"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "S3-${var.video_bucket_id}"

    forwarded_values {
      query_string = false
      headers = ["Origin"]
      cookies {
        forward = "none"
      }
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = 86400    # 1 day
    max_ttl                = 31536000 # 1 year
    compress               = true
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = {
    Name = "${var.project_name}-video-cdn"
  }
}

# CloudFront Distribution for Frontend
resource "aws_cloudfront_distribution" "frontend" {
  enabled             = true
  comment             = "${var.project_name} Frontend CDN"
  default_root_object = "index.html"
  price_class         = "PriceClass_100"

  origin {
    domain_name = var.frontend_bucket_website
    origin_id   = "S3-${var.project_name}-frontend"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    allowed_methods  = ["GET", "HEAD", "OPTIONS"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "S3-${var.project_name}-frontend"

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = 3600 # 1 hour
    max_ttl                = 86400
    compress               = true
  }

  # Custom error response for SPA routing
  custom_error_response {
    error_code         = 404
    response_code      = 200
    response_page_path = "/index.html"
  }

  custom_error_response {
    error_code         = 403
    response_code      = 200
    response_page_path = "/index.html"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  tags = {
    Name = "${var.project_name}-frontend-cdn"
  }
}
