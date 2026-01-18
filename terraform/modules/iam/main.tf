resource "aws_iam_role" "this" {
  name = var.name

  assume_role_policy = var.assume_role_policy
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.this.name
  policy_arn = var.basic_execution_policy_arn
}

resource "aws_iam_role_policy" "sqs_send" {
  count = length(var.sqs_send_arns) > 0 ? 1 : 0

  name = "${var.name}-sqs-send"
  role = aws_iam_role.this.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "SQSSendMessage"
        Effect   = "Allow"
        Action   = [
          "sqs:SendMessage"
        ]
        Resource = var.sqs_send_arns
      }
    ]
  })
}

