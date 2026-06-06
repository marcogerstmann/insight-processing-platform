# -------------------------------
# Worker Lambda (Container Image)
# -------------------------------

resource "aws_ecr_repository" "worker" {
  name                 = "ipp-dev-worker"
  image_tag_mutability = "IMMUTABLE"

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

module "worker_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "ipp-dev-worker-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

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
      },
      {
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = module.ingest_queue.dlq_arn
      }
    ]
  })
}

resource "aws_iam_role_policy" "worker_ssm_read" {
  name = "ipp-dev-worker-ssm-read"
  role = module.worker_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:parameter/ipp/dev/anthropic/api_key"
      }
    ]
  })
}

resource "aws_iam_policy" "worker_dynamodb" {
  name = "${var.project}-${var.env}-worker-dynamodb"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "PutInsightIfAbsent"
        Effect   = "Allow"
        Action   = ["dynamodb:PutItem"]
        Resource = module.dynamodb_insights.table_arn
      },
      {
        Sid      = "UpdateInsightAfterEnrichment"
        Effect   = "Allow"
        Action   = ["dynamodb:UpdateItem"]
        Resource = module.dynamodb_insights.table_arn
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "worker_dynamodb" {
  role       = module.worker_lambda_role.role_name
  policy_arn = aws_iam_policy.worker_dynamodb.arn
}

module "worker_lambda" {
  source      = "../../modules/lambda-image"
  name        = "ipp-dev-worker"
  role_arn    = module.worker_lambda_role.role_arn
  image_uri   = var.worker_image_uri
  timeout     = 30
  memory_size = 256

  environment_variables = {
    TABLE_NAME_INSIGHTS = module.dynamodb_insights.table_name
    INGEST_DLQ_URL      = module.ingest_queue.dlq_url
    ANTHROPIC_API_KEY   = "ssm:/ipp/dev/anthropic/api_key"
  }

  depends_on = [aws_iam_role_policy.worker_ecr_pull, aws_ecr_repository_policy.worker]
}

resource "aws_lambda_event_source_mapping" "worker_from_sqs" {
  event_source_arn = module.ingest_queue.queue_arn
  function_name    = module.worker_lambda.function_arn

  batch_size = 1
  enabled    = true
}
