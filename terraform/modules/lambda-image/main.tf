resource "aws_lambda_function" "this" {
  function_name = var.name
  role          = var.role_arn

  package_type = "Image"
  image_uri    = var.image_uri

  timeout     = var.timeout
  memory_size = var.memory_size

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
