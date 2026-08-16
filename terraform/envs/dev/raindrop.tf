# ---------------------------------------
# Raindrop Poll Lambda (ZIP packaging)
# ---------------------------------------
# Raindrop has no push webhook (unlike Readwise), so this Lambda is invoked
# on a schedule instead of by API Gateway — see the EventBridge Scheduler
# resources below.

data "archive_file" "raindrop_poll_lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/raindrop-poll-lambda/bootstrap"
  output_path = "${path.module}/raindrop-poll-lambda.zip"
}

module "raindrop_poll_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "${var.project}-${var.env}-raindrop-poll-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"

  # Enqueues onto the same ingest queue the REST import path uses
  # (rest-api.tf) so both dedupe against each other downstream.
  sqs_send_arns = [
    module.ingest_queue.queue_arn
  ]
}

resource "aws_iam_role_policy" "raindrop_poll_ssm_read" {
  name = "${var.project}-${var.env}-raindrop-poll-ssm-read"
  role = module.raindrop_poll_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:parameter/${var.project}/${var.env}/raindrop/api_token"
      }
    ]
  })
}

module "raindrop_poll_lambda" {
  source           = "../../modules/lambda-zip"
  name             = "${var.project}-${var.env}-raindrop-poll"
  role_arn         = module.raindrop_poll_lambda_role.role_arn
  filename         = data.archive_file.raindrop_poll_lambda_zip.output_path
  source_code_hash = data.archive_file.raindrop_poll_lambda_zip.output_base64sha256
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 128
  # Higher than the webhook/REST lambdas' 10s: a full poll pages through up
  # to raindrop_poll_limit highlights against Raindrop's API in one run.
  timeout = 30

  environment_variables = {
    DEFAULT_TENANT_ID   = var.default_tenant_id
    INGEST_QUEUE_URL    = module.ingest_queue.queue_url
    RAINDROP_API_TOKEN  = "ssm:/${var.project}/${var.env}/raindrop/api_token"
    RAINDROP_POLL_LIMIT = tostring(var.raindrop_poll_limit)
  }
}

# -------------------------------------------------------------------
# EventBridge Scheduler — invokes the poll Lambda on a recurring
# cadence. This is a schedule (aws_scheduler_schedule, its own service
# namespace, scheduler.amazonaws.com), not the domain-events bus the
# eventbridge/ module wraps (module "domain_events_bus" in events.tf) —
# that module fits a bus, not a schedule, and has exactly one caller
# here, so the schedule is defined directly rather than generalizing it.
# -------------------------------------------------------------------

data "aws_iam_policy_document" "scheduler_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "raindrop_poll_scheduler" {
  name               = "${var.project}-${var.env}-raindrop-poll-scheduler-role"
  assume_role_policy = data.aws_iam_policy_document.scheduler_assume_role.json
}

resource "aws_iam_role_policy" "raindrop_poll_scheduler_invoke" {
  name = "${var.project}-${var.env}-raindrop-poll-scheduler-invoke"
  role = aws_iam_role.raindrop_poll_scheduler.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["lambda:InvokeFunction"]
        Resource = module.raindrop_poll_lambda.lambda_arn
      }
    ]
  })
}

resource "aws_scheduler_schedule" "raindrop_poll" {
  name       = "${var.project}-${var.env}-raindrop-poll"
  group_name = "default"

  flexible_time_window {
    mode = "OFF"
  }

  # Cadence is a cost/noise knob, not a capacity one: the free tier and
  # Raindrop's 120 req/min rate limit are nowhere near binding at any
  # sane interval. Configurable via var.raindrop_poll_interval_hours.
  schedule_expression = "rate(${var.raindrop_poll_interval_hours} hours)"

  target {
    arn      = module.raindrop_poll_lambda.lambda_arn
    role_arn = aws_iam_role.raindrop_poll_scheduler.arn
  }
}

resource "aws_lambda_permission" "allow_scheduler_invoke_raindrop_poll" {
  statement_id  = "AllowSchedulerInvoke"
  action        = "lambda:InvokeFunction"
  function_name = module.raindrop_poll_lambda.lambda_function_name
  principal     = "scheduler.amazonaws.com"
  source_arn    = aws_scheduler_schedule.raindrop_poll.arn
}
