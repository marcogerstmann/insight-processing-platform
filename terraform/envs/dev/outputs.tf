output "webhook_url" {
  value = "${module.readwise_webhook_api.api_endpoint}/readwise/webhook"
}

output "worker_ecr_repository_url" {
  value = aws_ecr_repository.worker.repository_url
}
