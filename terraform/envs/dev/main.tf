locals {
  project = var.project
  name    = "${local.project}-ingest"
}

# -----------------------------
# Ingest Lambda (ZIP packaging)
# -----------------------------
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/ingest-lambda/bootstrap"
  output_path = "${path.module}/lambda.zip"
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

# -----------------------------
# SQS Queue (ingest events)
# -----------------------------
module "ingest_queue" {
  source                     = "../../modules/sqs"
  name                       = "ipp-dev-ingest-events"
  visibility_timeout_seconds = 120
  max_receive_count          = 5
}

# -----------------------------
# IAM Role: Ingest Lambda (send to SQS)
# -----------------------------
module "ingest_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "ipp-dev-ingest-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"

  sqs_send_arns = [
    module.ingest_queue.queue_arn
  ]
}

# -----------------------------
# Ingest Lambda Function (ZIP)
# -----------------------------
module "lambda" {
  source                = "../../modules/lambda-zip"
  name                  = local.name
  role_arn              = module.ingest_lambda_role.role_arn
  filename              = data.archive_file.lambda_zip.output_path
  source_code_hash      = data.archive_file.lambda_zip.output_base64sha256
  handler               = "bootstrap"
  runtime               = "provided.al2"
  memory_size           = 128
  timeout               = 10
  log_retention_in_days = 14

  environment_variables = {
    DEFAULT_TENANT_ID = "test-tenant-id"
    INGEST_QUEUE_URL  = module.ingest_queue.queue_url
  }
}

# -----------------------------
# API Gateway -> Ingest Lambda
# -----------------------------
module "api" {
  source            = "../../modules/api-gateway"
  name              = "${local.name}-api"
  lambda_invoke_arn = module.lambda.lambda_arn
}

resource "aws_lambda_permission" "allow_apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = module.lambda.lambda_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${module.api.execution_arn}/*/*"
}

# -----------------------------
# Worker Lambda (Container Image)
# -----------------------------

# ECR Repository for worker image
resource "aws_ecr_repository" "worker" {
  name                 = "${local.name}-worker"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

# IAM Role: Worker Lambda (consume from SQS)
module "worker_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "ipp-dev-worker-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Inline policy to allow SQS consumption (minimal permissions)
resource "aws_iam_role_policy" "worker_sqs_consume" {
  name = "ipp-dev-worker-sqs-consume"
  role = module.worker_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes",
          "sqs:ChangeMessageVisibility"
        ]
        Resource = module.ingest_queue.queue_arn
      }
    ]
  })
}

# Worker Lambda Function (Image)
variable "worker_image_uri" {
  type        = string
  description = "Full ECR image URI for the worker Lambda"
}

resource "aws_lambda_function" "worker" {
  function_name = "${local.name}-worker"
  role          = module.worker_lambda_role.role_arn

  package_type = "Image"
  image_uri    = var.worker_image_uri

  timeout     = 30
  memory_size = 256

  environment {
    variables = {}
  }

  depends_on = [
    aws_iam_role_policy.worker_sqs_consume
  ]
}

# SQS -> Worker Lambda trigger
resource "aws_lambda_event_source_mapping" "worker_from_sqs" {
  event_source_arn = module.ingest_queue.queue_arn
  function_name    = aws_lambda_function.worker.arn

  batch_size = 1
  enabled    = true
}
