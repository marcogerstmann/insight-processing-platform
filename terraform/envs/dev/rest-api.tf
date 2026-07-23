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
}

resource "aws_iam_role_policy" "rest_dynamodb" {
  name = "${var.project}-${var.env}-rest-dynamodb"
  role = module.rest_lambda_role.role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:Query", "dynamodb:PutItem"]
      Resource = module.dynamodb_insights.table_arn
    }]
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
    TABLE_NAME_INSIGHTS  = module.dynamodb_insights.table_name
    COGNITO_USER_POOL_ID = aws_cognito_user_pool.rest_api.id
    COGNITO_CLIENT_ID    = aws_cognito_user_pool_client.rest_api.id
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

# -----------------------------
# API Gateway (HTTPv2) + JWT Authorizer
# -----------------------------
resource "aws_apigatewayv2_api" "rest" {
  name          = "${var.project}-${var.env}-rest-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins = var.web_app_origins
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
  route_key = "GET /v1/tenants/{tenantID}/insights"

  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito_jwt.id

  target = "integrations/${aws_apigatewayv2_integration.rest_lambda.id}"
}

resource "aws_apigatewayv2_route" "post_insights" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "POST /v1/tenants/{tenantID}/insights"

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
