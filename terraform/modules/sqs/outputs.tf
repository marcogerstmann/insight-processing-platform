output "queue_url" {
  description = "Main queue URL"
  value       = aws_sqs_queue.main.url
}

output "queue_arn" {
  description = "Main queue ARN"
  value       = aws_sqs_queue.main.arn
}

output "dlq_url" {
  description = "DLQ URL"
  value       = aws_sqs_queue.dlq.url
}

output "dlq_arn" {
  description = "DLQ ARN"
  value       = aws_sqs_queue.dlq.arn
}

output "queue_name" {
  description = "Main queue name"
  value       = aws_sqs_queue.main.name
}
