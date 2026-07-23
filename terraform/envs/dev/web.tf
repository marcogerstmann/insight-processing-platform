# -----------------------------------------------------------------------------
# Static web app hosting (IPP-75, stretch).
#
# A private S3 bucket fronted by CloudFront via Origin Access Control (OAC): the
# bucket is never public, and only this distribution can read it. Uses the
# default *.cloudfront.net domain + certificate — custom domains/certs are out
# of scope. Content is uploaded + invalidated by .github/workflows/web-deploy.yml.
# -----------------------------------------------------------------------------

resource "aws_s3_bucket" "web" {
  # account_id suffix keeps the (globally unique) bucket name from colliding.
  bucket = "${var.project}-${var.env}-web-${data.aws_caller_identity.current.account_id}"

  tags = {
    Project = var.project
    Env     = var.env
  }
}

# All access is via CloudFront OAC, so lock the bucket down completely.
resource "aws_s3_bucket_public_access_block" "web" {
  bucket = aws_s3_bucket.web.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudfront_origin_access_control" "web" {
  name                              = "${var.project}-${var.env}-web-oac"
  description                       = "OAC for the ${var.project}-${var.env} web app bucket"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "web" {
  enabled             = true
  default_root_object = "index.html"
  comment             = "${var.project}-${var.env} web app"
  # US/Canada/Europe edge locations only — cheapest tier, enough for a demo.
  price_class = "PriceClass_100"

  origin {
    domain_name              = aws_s3_bucket.web.bucket_regional_domain_name
    origin_id                = "web-s3"
    origin_access_control_id = aws_cloudfront_origin_access_control.web.id
  }

  default_cache_behavior {
    target_origin_id       = "web-s3"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    # AWS managed "CachingOptimized" policy — a sensible default without inline
    # cache tuning (out of scope). Deploys invalidate the cache to stay fresh.
    cache_policy_id = "658327ea-f89d-4fab-a63d-7e88639e58f6"
  }

  # Single-page app: serve index.html for unknown paths so a deep link or a
  # refresh doesn't surface S3's 403/404.
  custom_error_response {
    error_code         = 403
    response_code      = 200
    response_page_path = "/index.html"
  }
  custom_error_response {
    error_code         = 404
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
    Project = var.project
    Env     = var.env
  }
}

# Allow only this CloudFront distribution to read objects from the bucket.
data "aws_iam_policy_document" "web_bucket" {
  statement {
    sid       = "AllowCloudFrontOACRead"
    effect    = "Allow"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.web.arn}/*"]

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.web.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "web" {
  bucket = aws_s3_bucket.web.id
  policy = data.aws_iam_policy_document.web_bucket.json
}
