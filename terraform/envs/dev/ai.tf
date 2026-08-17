# -------------------------------
# AI Lambda (Container Image) — subscribes to InsightEnriched
# -------------------------------
#
# EventBridge -> this service's own queue+DLQ (event-subscription module,
# EVT 4) -> this Lambda. See services/ai/README.md and ADR-014.
#
# Bootstrapping a brand-new image-based service hits the same chicken-and-egg
# problem github-actions.tf's OIDC comment describes: `make deploy` pushes an
# image before applying Terraform, but the ECR repo and the CI role's push
# permission for it are themselves created by that same apply. The one-time
# fix (as it was here): a local, Docker-free, targeted apply for just the new
# repo + lifecycle policy + `aws_iam_role_policy.github_actions` —
#   terraform apply -target=aws_ecr_repository.ai \
#     -target=aws_ecr_repository_policy.ai -target=aws_ecr_lifecycle_policy.ai \
#     -target=aws_iam_role_policy.github_actions
# — after which `make deploy` in CI can push and apply the rest unaided.

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

# This service's own table (IPP-97) — pk = TENANT#<id>, sk = EMBEDDING#<insightID>.
# Not the shared insights table: nothing outside this service reads or
# writes an embedding, so it doesn't belong in storage.tf with the table
# the Go core owns.
module "dynamodb_ai_embeddings" {
  source = "../../modules/dynamodb"

  name = "${var.project}-ai-embeddings"

  tags = {
    Project = var.project
    Env     = var.env
  }
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
# services/ai/README.md. IPP-97's embedding step is the first caller
# (get_by_id, to load the text to embed), but there is exactly one
# execution role for the whole service, so this is the one place that
# boundary is drawn.
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

# Write access to this service's own embeddings table (IPP-97) — a
# separate policy from ai_dynamodb_read above, which is scoped to the
# shared insights table and must stay GetItem/Query only.
resource "aws_iam_role_policy" "ai_embeddings_write" {
  name = "${var.project}-${var.env}-ai-embeddings-write"
  role = module.ai_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "PutEmbedding"
        Effect   = "Allow"
        Action   = ["dynamodb:PutItem"]
        Resource = module.dynamodb_ai_embeddings.table_arn
      }
    ]
  })
}

# Same pattern as worker.tf's worker_ssm_read for ANTHROPIC_API_KEY — the
# parameter itself is created out of band, not by Terraform.
resource "aws_iam_role_policy" "ai_voyage_ssm_read" {
  name = "${var.project}-${var.env}-ai-voyage-ssm-read"
  role = module.ai_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:parameter/${var.project}/${var.env}/voyage/api_key"
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
    TABLE_NAME_INSIGHTS     = module.dynamodb_insights.table_name
    TABLE_NAME_EMBEDDINGS   = module.dynamodb_ai_embeddings.table_name
    VOYAGE_API_KEY          = "ssm:/${var.project}/${var.env}/voyage/api_key"
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
