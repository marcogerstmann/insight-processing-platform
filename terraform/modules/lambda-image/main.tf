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

resource "aws_cloudwatch_log_group" "this" {
  count             = 1
  name              = "/aws/lambda/${aws_lambda_function.this.function_name}"
  retention_in_days = 14
}
