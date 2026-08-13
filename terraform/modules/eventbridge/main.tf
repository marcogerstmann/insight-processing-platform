resource "aws_cloudwatch_event_bus" "this" {
  name = var.name
  tags = var.tags
}
