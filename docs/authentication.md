# Authentication

The REST API is secured with a Cognito User Pool. API Gateway validates the JWT before the Lambda is ever invoked.

## One-time: read Terraform outputs

The S3 backend requires explicit credentials (it doesn't resolve SSO automatically). Run this once per shell session before any `terraform output` call:

```bash
eval "$(aws configure export-credentials --format env)"

POOL_ID=$(terraform -chdir=terraform/envs/dev output -raw cognito_user_pool_id)
CLIENT_ID=$(terraform -chdir=terraform/envs/dev output -raw cognito_client_id)
API_URL=$(terraform -chdir=terraform/envs/dev output -raw rest_api_endpoint)
```

## One-time: create a user

```bash
aws cognito-idp admin-create-user \
  --user-pool-id $POOL_ID \
  --username you@example.com \
  --temporary-password Temp1234!

aws cognito-idp admin-set-user-password \
  --user-pool-id $POOL_ID \
  --username you@example.com \
  --password '<your-password>' \
  --permanent
```

## Get a token

Passwords with special characters (`"`, `!`, etc.) break the AWS CLI shorthand parser — use JSON format for `--auth-parameters`:

```bash
TOKEN=$(aws cognito-idp initiate-auth \
  --auth-flow USER_PASSWORD_AUTH \
  --auth-parameters '{"USERNAME":"you@example.com","PASSWORD":"<your-password>"}' \
  --client-id $CLIENT_ID \
  --query 'AuthenticationResult.IdToken' \
  --output text)
```

The `IdToken` is a JWT valid for 1 hour. Use `AuthenticationResult.RefreshToken` to get a new one without re-entering credentials.

## Call the API

```bash
curl -H "Authorization: Bearer $TOKEN" \
  $API_URL/tenants/<tenantID>/insights
```
