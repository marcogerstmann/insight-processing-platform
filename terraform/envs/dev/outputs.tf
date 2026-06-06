output "webhook_url" {
  value = "${module.readwise_webhook_api.api_endpoint}/readwise/webhook"
}

output "worker_ecr_repository_url" {
  value = aws_ecr_repository.worker.repository_url
}

output "rest_api_endpoint" {
  value = aws_apigatewayv2_api.rest.api_endpoint
}

output "cognito_client_id" {
  value = aws_cognito_user_pool_client.rest_api.id
}

output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.rest_api.id
}
