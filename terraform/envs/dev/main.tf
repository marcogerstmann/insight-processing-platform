locals {
  project = var.project 
  name    = "${local.project}-ingest"
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
  role_arn          = module.ingest_lambda_role.role_arn
  filename          = data.archive_file.lambda_zip.output_path
  source_code_hash  = data.archive_file.lambda_zip.output_base64sha256
  handler           = "bootstrap"
  runtime           = "provided.al2"
  memory_size       = 128
  timeout           = 10
  log_retention_in_days = 14
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

module "ingest_queue" {
  source = "../../modules/sqs"
  name   = "ipp-dev-ingest-events"
  visibility_timeout_seconds = 120
  max_receive_count          = 5
}

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

module "ingest_lambda_role" {
  source = "../../modules/iam"
  name                    = "ipp-dev-ingest-lambda-role"
  assume_role_policy      = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
  sqs_send_arns = [
    module.ingest_queue.queue_arn
  ]
}
