output "webhook_url" {
  description = "Full URL for the Readwise webhook endpoint"
  value       = "${module.readwise_webhook_api.api_endpoint}/readwise/webhook"
}

output "worker_ecr_repository_url" {
  description = "ECR repository URL for pushing worker container images"
  value       = aws_ecr_repository.worker.repository_url
}

output "rest_api_endpoint" {
  description = "Base URL of the REST API Gateway"
  value       = aws_apigatewayv2_api.rest.api_endpoint
}

output "cognito_client_id" {
  description = "Cognito app client ID for authenticating REST API requests"
  value       = aws_cognito_user_pool_client.rest_api.id
}

output "cognito_user_pool_id" {
  description = "Cognito user pool ID"
  value       = aws_cognito_user_pool.rest_api.id
}

output "web_url" {
  description = "Public URL of the deployed web app"
  value       = "https://${aws_cloudfront_distribution.web.domain_name}"
}

output "web_bucket" {
  description = "S3 bucket hosting the web app's static build"
  value       = aws_s3_bucket.web.bucket
}

output "web_cloudfront_distribution_id" {
  description = "CloudFront distribution ID (used to invalidate cache on deploy)"
  value       = aws_cloudfront_distribution.web.id
}
