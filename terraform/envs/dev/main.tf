locals {
  project = "ipp"
  name    = "${local.project}-ingress"
}

# Package Lambda (zip)
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/ingest-lambda/bootstrap"
  output_path = "${path.module}/lambda.zip"
}

module "iam" {
  source               = "../../modules/iam"
  name                 = local.name
  assume_role_policy   = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action = "sts:AssumeRole"
    }]
  })
}

module "lambda" {
  source            = "../../modules/lambda"
  name              = local.name
  role_arn          = module.iam.role_arn
  filename          = data.archive_file.lambda_zip.output_path
  source_code_hash  = data.archive_file.lambda_zip.output_base64sha256
  handler           = "bootstrap"
  runtime           = "provided.al2"
  memory_size       = 128
  timeout           = 5
}

module "api" {
  source          = "../../modules/api-gateway"
  name            = "${local.name}-api"
  lambda_invoke_arn = module.lambda.lambda_arn
}

resource "aws_lambda_permission" "allow_apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = module.lambda.lambda_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${module.api.execution_arn}/*/*"
}
