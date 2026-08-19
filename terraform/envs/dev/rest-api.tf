# -------------------------------
# REST API Lambda (ZIP packaging)
# -------------------------------

data "archive_file" "rest_lambda_zip" {
  type        = "zip"
  source_file = "${path.module}/../../../cmd/rest-lambda/bootstrap"
  output_path = "${path.module}/rest-lambda.zip"
}

module "rest_lambda_role" {
  source                     = "../../modules/iam"
  name                       = "${var.project}-${var.env}-rest-lambda-role"
  assume_role_policy         = data.aws_iam_policy_document.lambda_assume_role.json
  basic_execution_policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"

  # Readwise import (POST /v1/readwise/import) enqueues
  # onto the same ingest queue the webhook uses (terraform/envs/dev/readwise.tf)
  # so the two paths dedupe against each other downstream.
  sqs_send_arns = [
    module.ingest_queue.queue_arn
  ]
}

resource "aws_iam_role_policy" "rest_dynamodb" {
  name = "${var.project}-${var.env}-rest-dynamodb"
  role = module.rest_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["dynamodb:Query", "dynamodb:PutItem", "dynamodb:GetItem"]
        Resource = module.dynamodb_insights.table_arn
      },
      {
        Sid      = "QueryTagIndex"
        Effect   = "Allow"
        Action   = ["dynamodb:Query"]
        Resource = "${module.dynamodb_insights.table_arn}/index/*"
      }
    ]
  })
}

resource "aws_iam_role_policy" "rest_eventbridge_publish" {
  name = "${var.project}-${var.env}-rest-eventbridge-publish"
  role = module.rest_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["events:PutEvents"]
        Resource = module.domain_events_bus.bus_arn
      }
    ]
  })
}

resource "aws_iam_role_policy" "rest_readwise_ssm_read" {
  name = "${var.project}-${var.env}-rest-readwise-ssm-read"
  role = module.rest_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:parameter/${var.project}/${var.env}/readwise/api_token"
      }
    ]
  })
}

# RAINDROP 3's POST /v1/raindrop/import endpoint needs this too — easy to
# wire only the poll Lambda (raindrop.tf) and be confused when the manual
# import 500s in dev.
resource "aws_iam_role_policy" "rest_raindrop_ssm_read" {
  name = "${var.project}-${var.env}-rest-raindrop-ssm-read"
  role = module.rest_lambda_role.role_name

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

module "rest_lambda" {
  source           = "../../modules/lambda-zip"
  name             = "${var.project}-${var.env}-rest"
  role_arn         = module.rest_lambda_role.role_arn
  filename         = data.archive_file.rest_lambda_zip.output_path
  source_code_hash = data.archive_file.rest_lambda_zip.output_base64sha256
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  memory_size      = 128
  timeout          = 10

  environment_variables = {
    TABLE_NAME_INSIGHTS     = module.dynamodb_insights.table_name
    COGNITO_USER_POOL_ID    = aws_cognito_user_pool.rest_api.id
    COGNITO_CLIENT_ID       = aws_cognito_user_pool_client.rest_api.id
    COGNITO_AGENT_CLIENT_ID = aws_cognito_user_pool_client.agent.id
    INGEST_QUEUE_URL        = module.ingest_queue.queue_url
    DOMAIN_EVENTS_BUS_NAME  = module.domain_events_bus.bus_name
    # Falls back to this when a request doesn't pass its own "token"; must be
    # created out-of-band the same way readwise/webhook_secret is (no
    # aws_ssm_parameter resource in this repo — see readwise.tf).
    READWISE_API_TOKEN = "ssm:/${var.project}/${var.env}/readwise/api_token"
    RAINDROP_API_TOKEN = "ssm:/${var.project}/${var.env}/raindrop/api_token"
  }
}

# -----------------------------
# Cognito User Pool (auth)
# -----------------------------

# aws_iam_role_policy.github_actions and this user pool have no natural
# resource dependency, so Terraform applies them concurrently by default.
# When a Cognito-permission change lands in the same apply as a Cognito change
# (e.g. AddCustomAttributes), the CI role's updated inline policy can still be
# propagating in IAM when the Cognito call fires, producing a flaky
# AccessDeniedException. Force ordering plus a short wait for propagation.
resource "time_sleep" "cognito_iam_propagation" {
  depends_on      = [aws_iam_role_policy.github_actions]
  create_duration = "15s"
}

resource "aws_cognito_user_pool" "rest_api" {
  depends_on = [time_sleep.cognito_iam_propagation]

  name = "${var.project}-${var.env}-rest-api-users"

  password_policy {
    minimum_length    = 12
    require_uppercase = true
    require_numbers   = true
    require_symbols   = true
  }

  # Carries the tenant ID assigned to each user. Only present in ID tokens,
  # which the REST API's Gin middleware requires for that reason.
  schema {
    name                = "tenant_id"
    attribute_data_type = "String"
    mutable             = true
    required            = false

    string_attribute_constraints {
      min_length = 1
      max_length = 256
    }
  }
}

resource "aws_cognito_user_pool_client" "rest_api" {
  name         = "${var.project}-${var.env}-rest-api-client"
  user_pool_id = aws_cognito_user_pool.rest_api.id

  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH",
  ]
}

# -----------------------------------------------
# Machine-to-machine auth for the AI service (IPP-94)
# -----------------------------------------------
#
# The AI service authenticates as itself via OAuth2 client_credentials, not
# as any user — see internal/adapters/inbound/http/rest/auth/cognito.go's
# authenticateService. That grant needs a resource server (to define a
# custom scope), a domain (Cognito's /oauth2/token endpoint is only served
# under the user pool's domain, unlike the AdminInitiateAuth-based flow the
# human client above uses, which needs no domain), and a second app client.
#
# All three explicitly depend on time_sleep.cognito_iam_propagation, the
# same guard aws_cognito_user_pool.rest_api uses above: referencing
# aws_cognito_user_pool.rest_api.id alone doesn't force that ordering,
# because that pool already exists from an earlier apply and has no
# changes of its own to wait on here. The first apply of these resources
# hit exactly this race — cognito-idp:CreateResourceServer /
# CreateUserPoolDomain denied ~10s after the IAM policy granting them
# finished updating, IAM's own propagation lag.

resource "aws_cognito_resource_server" "agent" {
  depends_on = [time_sleep.cognito_iam_propagation]

  identifier   = "ipp"
  name         = "${var.project}-${var.env}-agent-api"
  user_pool_id = aws_cognito_user_pool.rest_api.id

  scope {
    scope_name        = "agent.write"
    scope_description = "Write access for the AI service's relationship-discovery agent (REL 4, IPP-100)"
  }
}

# Prefix must be globally unique across all AWS accounts in the region, not
# just this account — if this collides on apply, rename it.
resource "aws_cognito_user_pool_domain" "rest_api" {
  depends_on = [time_sleep.cognito_iam_propagation]

  domain       = "${var.project}-${var.env}-agent-auth"
  user_pool_id = aws_cognito_user_pool.rest_api.id
}

resource "aws_cognito_user_pool_client" "agent" {
  name         = "${var.project}-${var.env}-agent-client"
  user_pool_id = aws_cognito_user_pool.rest_api.id

  generate_secret                      = true
  allowed_oauth_flows                  = ["client_credentials"]
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_scopes                 = ["${aws_cognito_resource_server.agent.identifier}/agent.write"]
}

# Generated by Cognito, not hand-typed, so it's already in this resource's
# state regardless — unlike readwise/openai's out-of-band secrets (see
# ai.tf), storing it via a managed parameter here doesn't newly expose
# anything a plan/apply doesn't already see.
resource "aws_ssm_parameter" "agent_client_secret" {
  name  = "/${var.project}/${var.env}/agent/client_secret"
  type  = "SecureString"
  value = aws_cognito_user_pool_client.agent.client_secret
}

# -----------------------------
# API Gateway (HTTPv2) + JWT Authorizer
# -----------------------------
resource "aws_apigatewayv2_api" "rest" {
  name          = "${var.project}-${var.env}-rest-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins = concat(var.web_app_origins, [
      "https://${aws_cloudfront_distribution.web.domain_name}",
      "https://${var.domain_name}",
    ])
    allow_methods = ["GET", "POST"]
    allow_headers = ["Authorization", "Content-Type"]
  }
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
    # Both app clients: a human's ID token carries the rest_api client ID in
    # "aud"; the AI service's access token has no "aud" claim at all, so API
    # Gateway's Cognito-specific JWT authorizer matches "audience" against
    # "client_id" instead — the agent client ID must be in this same list
    # for that fallback to accept it. See cognito.go's authenticate for the
    # same aud-vs-client_id distinction made again one layer down, in Gin.
    audience = [aws_cognito_user_pool_client.rest_api.id, aws_cognito_user_pool_client.agent.id]
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
  route_key = "GET /v1/insights"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "post_insights" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "POST /v1/insights"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "get_tags" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "GET /v1/tags"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "post_readwise_import" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "POST /v1/readwise/import"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "post_raindrop_import" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "POST /v1/raindrop/import"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "post_relationships" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "POST /v1/tenants/{tenantID}/insights/{insightID}/relationships"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "get_relationships" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "GET /v1/tenants/{tenantID}/insights/{insightID}/relationships"

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
