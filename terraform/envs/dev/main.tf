locals {
  project = var.project
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# -----------------------------
# Readwise Webhook Lambda (ZIP packaging)
# -----------------------------
data "archive_file" "readwise_lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/readwise-lambda/bootstrap"
  output_path = "${path.module}/readwise-lambda.zip"
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
# IAM Role: Readwise Lambda (send to SQS)
# -----------------------------
module "readwise_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "ipp-dev-readwise-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"

  sqs_send_arns = [
    module.ingest_queue.queue_arn
  ]
}

# -----------------------------
# Readwise Webhook Lambda Function (ZIP)
# -----------------------------
module "readwise_lambda" {
  source           = "../../modules/lambda-zip"
  name             = "ipp-readwise"
  role_arn         = module.readwise_lambda_role.role_arn
  filename         = data.archive_file.readwise_lambda_zip.output_path
  source_code_hash = data.archive_file.readwise_lambda_zip.output_base64sha256
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 128
  timeout          = 10

  environment_variables = {
    DEFAULT_TENANT_ID           = "test-tenant-id"
    INGEST_QUEUE_URL            = module.ingest_queue.queue_url
    READWISE_WEBHOOK_SECRET_SSM = "/ipp/dev/readwise/webhook_secret"
  }
}

# -----------------------------
# API Gateway -> Readwise Webhook Lambda
# -----------------------------
module "readwise_webhook_api" {
  source            = "../../modules/api-gateway"
  name              = "ipp-readwise-api"
  lambda_invoke_arn = module.readwise_lambda.lambda_arn
}

resource "aws_lambda_permission" "allow_readwise_apigw" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = module.readwise_lambda.lambda_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${module.readwise_webhook_api.execution_arn}/*/*"
}

# -----------------------------
# Worker Lambda (Container Image)
# -----------------------------

# ECR Repository for worker image
resource "aws_ecr_repository" "worker" {
  name                 = "ipp-dev-worker"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository_policy" "worker" {
  repository = aws_ecr_repository.worker.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "LambdaECRImageRetrievalPolicy"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
        Action = [
          "ecr:BatchGetImage",
          "ecr:GetDownloadUrlForLayer"
        ]
      }
    ]
  })
}

resource "aws_ecr_lifecycle_policy" "worker" {
  repository = aws_ecr_repository.worker.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 3 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 3
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

# IAM Role: Worker Lambda (consume from SQS)
module "worker_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "ipp-dev-worker-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Inline policy to allow ECR image pull (required for container image Lambda)
resource "aws_iam_role_policy" "worker_ecr_pull" {
  name = "ipp-dev-worker-ecr-pull"
  role = module.worker_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchGetImage",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchCheckLayerAvailability"
        ]
        Resource = aws_ecr_repository.worker.arn
      }
    ]
  })
}

# Inline policy to allow SQS consumption
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

# AWS Systems Manager Parameter Store for secrets
resource "aws_iam_role_policy" "readwise_ssm_read" {
  name = "ipp-dev-readwise-ssm-read"
  role = module.readwise_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:parameter/ipp/dev/readwise/webhook_secret"
      }
    ]
  })
}

# Worker Lambda Function (Image)
variable "worker_image_uri" {
  type        = string
  description = "Full ECR image URI for the worker Lambda"
}

module  "worker_lambda" {
  source      = "../../modules/lambda-image"
  name        = "ipp-dev-worker"
  role_arn    = module.worker_lambda_role.role_arn
  image_uri   = var.worker_image_uri
  timeout     = 30
  memory_size = 256

  environment_variables = {
    TABLE_NAME_INSIGHTS = module.dynamodb_insights.table_name
  }

  depends_on = [aws_iam_role_policy.worker_ecr_pull, aws_ecr_repository_policy.worker]
}

# SQS -> Worker Lambda trigger
resource "aws_lambda_event_source_mapping" "worker_from_sqs" {
  event_source_arn = module.ingest_queue.queue_arn
  function_name    = module.worker_lambda.function_arn

  batch_size = 1
  enabled    = true
}

# DynamoDB table for insights
module "dynamodb_insights" {
  source = "../../modules/dynamodb"

  name = "${var.project}-insights"

  tags = {
    Project = var.project
    Env     = var.env
  }
}

resource "aws_iam_policy" "worker_dynamodb" {
  name = "${var.project}-${var.env}-worker-dynamodb"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "PutInsightIfAbsent"
        Effect = "Allow"
        Action = ["dynamodb:PutItem"]
        Resource = module.dynamodb_insights.table_arn
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "worker_dynamodb" {
  role       = module.worker_lambda_role.role_name
  policy_arn = aws_iam_policy.worker_dynamodb.arn
}

# -----------------------------
# REST Lambda (ZIP packaging)
# -----------------------------
data "archive_file" "rest_lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/rest-lambda/bootstrap"
  output_path = "${path.module}/rest-lambda.zip"
}

module "rest_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "ipp-dev-rest-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "rest_dynamodb_query" {
  name = "ipp-dev-rest-dynamodb-query"
  role = module.rest_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:Query"]
      Resource = module.dynamodb_insights.table_arn
    }]
  })
}

module "rest_lambda" {
  source           = "../../modules/lambda-zip"
  name             = "ipp-dev-rest"
  role_arn         = module.rest_lambda_role.role_arn
  filename         = data.archive_file.rest_lambda_zip.output_path
  source_code_hash = data.archive_file.rest_lambda_zip.output_base64sha256
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 128
  timeout          = 10

  environment_variables = {
    TABLE_NAME_INSIGHTS = module.dynamodb_insights.table_name
  }
}

# -----------------------------
# Cognito User Pool (auth)
# -----------------------------
resource "aws_cognito_user_pool" "rest_api" {
  name = "ipp-dev-rest-api-users"

  password_policy {
    minimum_length    = 12
    require_uppercase = true
    require_numbers   = true
    require_symbols   = true
  }
}

resource "aws_cognito_user_pool_client" "rest_api" {
  name         = "ipp-dev-rest-api-client"
  user_pool_id = aws_cognito_user_pool.rest_api.id

  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH",
  ]
}

# -----------------------------
# API Gateway (REST) + JWT Authorizer
# -----------------------------
resource "aws_apigatewayv2_api" "rest" {
  name          = "ipp-dev-rest-api"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_stage" "rest_default" {
  api_id      = aws_apigatewayv2_api.rest.id
  name        = "$default"
  auto_deploy = true
}

resource "aws_apigatewayv2_authorizer" "cognito_jwt" {
  api_id           = aws_apigatewayv2_api.rest.id
  authorizer_type  = "JWT"
  identity_sources = ["$request.header.Authorization"]
  name             = "cognito-jwt"

  jwt_configuration {
    audience = [aws_cognito_user_pool_client.rest_api.id]
    issuer   = "https://cognito-idp.${data.aws_region.current.id}.amazonaws.com/${aws_cognito_user_pool.rest_api.id}"
  }
}

resource "aws_apigatewayv2_integration" "rest_lambda" {
  api_id                 = aws_apigatewayv2_api.rest.id
  integration_type       = "AWS_PROXY"
  integration_uri        = module.rest_lambda.lambda_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "get_insights" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "GET /tenants/{tenantID}/insights"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_lambda_permission" "allow_rest_apigw" {
  statement_id  = "AllowRestAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = module.rest_lambda.lambda_function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.rest.execution_arn}/*/*"
}

output "rest_api_endpoint" {
  value = aws_apigatewayv2_api.rest.api_endpoint
}

output "cognito_client_id" {
  value = aws_cognito_user_pool_client.rest_api.id
}

output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.rest_api.id
}
