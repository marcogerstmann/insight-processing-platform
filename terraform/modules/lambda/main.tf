resource "aws_lambda_function" "this" {
  function_name = var.name
  role          = var.role_arn

  filename         = var.filename
  source_code_hash = var.source_code_hash

  handler = var.handler
  runtime = var.runtime

  memory_size = var.memory_size
  timeout     = var.timeout
  
  architectures = ["arm64"]

  environment {
    variables = var.environment_variables
  }
}

resource "aws_cloudwatch_log_group" "this" {
  count = var.manage_log_group ? 1 : 0

  name              = "/aws/lambda/${aws_lambda_function.this.function_name}"
  retention_in_days = var.log_retention_in_days
}
