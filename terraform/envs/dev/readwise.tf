# ---------------------------------------
# Readwise Webhook Lambda (ZIP packaging)
# ---------------------------------------

data "archive_file" "readwise_lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/readwise-lambda/bootstrap"
  output_path = "${path.module}/readwise-lambda.zip"
}

module "ingest_queue" {
  source                     = "../../modules/sqs"
  name                       = "${var.project}-${var.env}-ingest-events"
  visibility_timeout_seconds = 120
  max_receive_count          = 5
}

module "readwise_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "${var.project}-${var.env}-readwise-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"

  sqs_send_arns = [
    module.ingest_queue.queue_arn
  ]
}

resource "aws_iam_role_policy" "readwise_ssm_read" {
  name = "${var.project}-${var.env}-readwise-ssm-read"
  role = module.readwise_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:parameter/${var.project}/${var.env}/readwise/webhook_secret"
      }
    ]
  })
}

module "readwise_lambda" {
  source           = "../../modules/lambda-zip"
  name             = "${var.project}-${var.env}-readwise"
  role_arn         = module.readwise_lambda_role.role_arn
  filename         = data.archive_file.readwise_lambda_zip.output_path
  source_code_hash = data.archive_file.readwise_lambda_zip.output_base64sha256
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 128
  timeout          = 10

  environment_variables = {
    DEFAULT_TENANT_ID       = var.default_tenant_id
    INGEST_QUEUE_URL        = module.ingest_queue.queue_url
    READWISE_WEBHOOK_SECRET = "ssm:/${var.project}/${var.env}/readwise/webhook_secret"
  }
}

module "readwise_webhook_api" {
  source            = "../../modules/api-gateway"
  name              = "${var.project}-${var.env}-readwise-api"
  lambda_invoke_arn = module.readwise_lambda.lambda_arn
}

resource "aws_lambda_permission" "allow_readwise_apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = module.readwise_lambda.lambda_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${module.readwise_webhook_api.execution_arn}/*/*"
}
