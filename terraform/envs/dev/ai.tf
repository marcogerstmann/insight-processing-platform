# -------------------------------
# AI Lambda (Container Image) — subscribes to InsightEnriched
# -------------------------------
#
# EventBridge -> this service's own queue+DLQ (event-subscription module,
# EVT 4) -> this Lambda. See services/ai/README.md and ADR-014.

resource "aws_ecr_repository" "ai" {
  name                 = "${var.project}-${var.env}-ai"
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository_policy" "ai" {
  repository = aws_ecr_repository.ai.name

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

resource "aws_ecr_lifecycle_policy" "ai" {
  repository = aws_ecr_repository.ai.name

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

module "ai_subscription" {
  source = "../../modules/event-subscription"

  bus_name        = module.domain_events_bus.bus_name
  subscriber_name = "${var.project}-${var.env}-ai"
  detail_types    = ["InsightEnriched"]

  tags = {
    Project = var.project
    Env     = var.env
  }
}

module "ai_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "${var.project}-${var.env}-ai-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "ai_ecr_pull" {
  name = "${var.project}-${var.env}-ai-ecr-pull"
  role = module.ai_lambda_role.role_name

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
        Resource = aws_ecr_repository.ai.arn
      }
    ]
  })
}

resource "aws_iam_role_policy" "ai_sqs_consume" {
  name = "${var.project}-${var.env}-ai-sqs-consume"
  role = module.ai_lambda_role.role_name

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
        Resource = module.ai_subscription.queue_arn
      },
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = module.ai_subscription.dlq_arn
      }
    ]
  })
}

# Read-only, per IPP-93: the AI service's insight repository only ever calls
# GetItem/Query. No PutItem, UpdateItem, or DeleteItem — see
# services/ai/README.md. Nothing in this Lambda calls it yet (IPP-95's
# handler does no more than log), but there is exactly one execution role
# for the whole service, so this is the one place that boundary is drawn.
resource "aws_iam_role_policy" "ai_dynamodb_read" {
  name = "${var.project}-${var.env}-ai-dynamodb-read"
  role = module.ai_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "GetInsight"
        Effect   = "Allow"
        Action   = ["dynamodb:GetItem"]
        Resource = module.dynamodb_insights.table_arn
      },
      {
        Sid    = "QueryInsightsAndTagIndex"
        Effect = "Allow"
        Action = ["dynamodb:Query"]
        Resource = [
          module.dynamodb_insights.table_arn,
          "${module.dynamodb_insights.table_arn}/index/*"
        ]
      }
    ]
  })
}

module "ai_lambda" {
  source      = "../../modules/lambda-image"
  name        = "${var.project}-${var.env}-ai"
  role_arn    = module.ai_lambda_role.role_arn
  image_uri   = var.ai_image_uri
  timeout     = 30
  memory_size = 256

  environment_variables = {
    AI_SUBSCRIPTION_DLQ_URL = module.ai_subscription.dlq_url
  }

  depends_on = [aws_iam_role_policy.ai_ecr_pull, aws_ecr_repository_policy.ai]
}

resource "aws_lambda_event_source_mapping" "ai_from_subscription_queue" {
  event_source_arn = module.ai_subscription.queue_arn
  function_name    = module.ai_lambda.function_arn

  # Must stay 1 — see the batch_size comment in
  # services/ai/src/ipp_ai/adapters/inbound/event_subscription.py (same
  # constraint as the Go worker, ADR-009).
  batch_size = 1
  enabled    = true
}
