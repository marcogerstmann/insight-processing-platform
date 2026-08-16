# Subscribes a consumer to the domain events bus via its own queue + DLQ, so
# one subscriber's failures and retries never touch another's.

resource "aws_cloudwatch_event_rule" "this" {
  name           = "${var.subscriber_name}-rule"
  event_bus_name = var.bus_name

  event_pattern = jsonencode({
    source      = ["ipp.core"]
    detail-type = var.detail_types
  })

  tags = var.tags
}

module "queue" {
  source            = "../sqs"
  name              = var.subscriber_name
  max_receive_count = var.max_receive_count
  tags              = var.tags
}

# Without this, the rule shows healthy and matched events simply vanish -
# EventBridge needs explicit permission to SendMessage to a queue it targets.
resource "aws_sqs_queue_policy" "allow_eventbridge" {
  queue_url = module.queue.queue_url

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AllowEventBridgeSendMessage"
        Effect    = "Allow"
        Principal = { Service = "events.amazonaws.com" }
        Action    = "sqs:SendMessage"
        Resource  = module.queue.queue_arn
        Condition = {
          ArnEquals = {
            "aws:SourceArn" = aws_cloudwatch_event_rule.this.arn
          }
        }
      }
    ]
  })
}

resource "aws_cloudwatch_event_target" "to_queue" {
  rule           = aws_cloudwatch_event_rule.this.name
  event_bus_name = var.bus_name
  arn            = module.queue.queue_arn
}
