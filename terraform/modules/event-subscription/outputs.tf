output "queue_arn" {
  description = "ARN of the subscriber's queue"
  value       = module.queue.queue_arn
}

output "queue_url" {
  description = "URL of the subscriber's queue"
  value       = module.queue.queue_url
}

output "dlq_arn" {
  description = "ARN of the subscriber's DLQ"
  value       = module.queue.dlq_arn
}

output "dlq_url" {
  description = "URL of the subscriber's DLQ"
  value       = module.queue.dlq_url
}
