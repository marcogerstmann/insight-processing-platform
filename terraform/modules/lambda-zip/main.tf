resource "aws_lambda_function" "this" {
  function_name = var.name
  role          = var.role_arn

  filename         = var.filename
  source_code_hash = var.source_code_hash

  handler = var.handler
  runtime = var.runtime

  memory_size = var.memory_size
  timeout     = var.timeout

  architectures = ["x86_64"]

  environment {
    variables = var.environment_variables
  }
}

moved {
  from = aws_cloudwatch_log_group.this[0]
  to   = aws_cloudwatch_log_group.this
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/aws/lambda/${aws_lambda_function.this.function_name}"
  retention_in_days = 14
}
