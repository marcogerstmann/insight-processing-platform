# -----------------------------------------------------------------------------
# Custom domain: ipp.marcogerstmann.com (web app) / api.ipp.marcogerstmann.com
# (REST API). DNS lives at Cloudflare, not Route53, so ACM's DNS validation
# can't be fully automated here:
#
#   1. `terraform apply` — creates both certs (PENDING_VALIDATION) and fails on
#      the CloudFront/API Gateway domain resources below, which require an
#      ISSUED cert.
#   2. Add the CNAMEs from the `acm_validation_records` output in Cloudflare,
#      DNS only (grey cloud) — proxying would hide the record ACM needs to see.
#   3. Once both certs show ISSUED in the ACM console, `terraform apply` again
#      to attach them.
# -----------------------------------------------------------------------------

resource "aws_acm_certificate" "web" {
  provider          = aws.us_east_1
  domain_name       = var.domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_acm_certificate" "api" {
  domain_name       = var.api_domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_apigatewayv2_domain_name" "rest" {
  domain_name = var.api_domain_name

  domain_name_configuration {
    certificate_arn = aws_acm_certificate.api.arn
    endpoint_type   = "REGIONAL"
    security_policy = "TLS_1_2"
  }
}

resource "aws_apigatewayv2_api_mapping" "rest" {
  api_id      = aws_apigatewayv2_api.rest.id
  domain_name = aws_apigatewayv2_domain_name.rest.id
  stage       = aws_apigatewayv2_stage.rest_default.id
}

output "acm_validation_records" {
  description = "DNS validation CNAMEs to create in Cloudflare (DNS only, not proxied)"
  value = {
    for cert in [aws_acm_certificate.web, aws_acm_certificate.api] :
    cert.domain_name => {
      name  = tolist(cert.domain_validation_options)[0].resource_record_name
      type  = tolist(cert.domain_validation_options)[0].resource_record_type
      value = tolist(cert.domain_validation_options)[0].resource_record_value
    }
  }
}

output "api_domain_cname_target" {
  description = "CNAME target for api.ipp.marcogerstmann.com in Cloudflare"
  value       = aws_apigatewayv2_domain_name.rest.domain_name_configuration[0].target_domain_name
}
