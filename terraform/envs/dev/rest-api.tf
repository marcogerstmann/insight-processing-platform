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

resource "aws_iam_role_policy" "rest_dynamodb_query" {
  name = "${var.project}-${var.env}-rest-dynamodb-query"
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
  name             = "${var.project}-${var.env}-rest"
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
  name = "${var.project}-${var.env}-rest-api-users"

  password_policy {
    minimum_length    = 12
    require_uppercase = true
    require_numbers   = true
    require_symbols   = true
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

resource "aws_apigatewayv2_route" "post_insights" {
  api_id    = aws_apigatewayv2_api.rest.id
  route_key = "POST /tenants/{tenantID}/insights"

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
