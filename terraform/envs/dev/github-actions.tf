# GitHub Actions OIDC federation — lets CI assume an IAM role without storing
# long-lived access keys as secrets. Apply once from local with `make tf-apply`,
# then GitHub Actions can manage itself going forward.
#
# If an OIDC provider for token.actions.githubusercontent.com already exists in
# this account (created by another project), import it instead of creating it:
#   terraform import aws_iam_openid_connect_provider.github \
#     arn:aws:iam::<ACCOUNT_ID>:oidc-provider/token.actions.githubusercontent.com

resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  # AWS validates GitHub's OIDC certificates via its own CAs — the thumbprint
  # field is required by the API but not used for validation on this provider.
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

data "aws_iam_policy_document" "github_actions_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    # Scope to pushes on main only — prevents PRs or other branches from deploying.
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:marcogerstmann/insight-processing-platform:ref:refs/heads/main"]
    }
  }
}

resource "aws_iam_role" "github_actions" {
  name               = "ipp-github-actions"
  assume_role_policy = data.aws_iam_policy_document.github_actions_assume_role.json
}

data "aws_iam_policy_document" "github_actions_permissions" {
  # ------------------------------------------------------------------
  # ECR — authenticate and push the worker container image
  # ------------------------------------------------------------------
  statement {
    sid     = "ECRAuth"
    effect  = "Allow"
    actions = ["ecr:GetAuthorizationToken"]
    # GetAuthorizationToken is account-scoped; cannot be restricted by resource.
    resources = ["*"]
  }

  statement {
    sid    = "ECRPush"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage",
      "ecr:BatchGetImage",
    ]
    resources = [aws_ecr_repository.worker.arn]
  }

  # ------------------------------------------------------------------
  # ECR repository management — Terraform manages the repo + lifecycle policy
  # ------------------------------------------------------------------
  statement {
    sid    = "ECRManage"
    effect = "Allow"
    actions = [
      "ecr:CreateRepository",
      "ecr:DeleteRepository",
      "ecr:DescribeRepositories",
      "ecr:PutLifecyclePolicy",
      "ecr:GetLifecyclePolicy",
      "ecr:DeleteLifecyclePolicy",
      "ecr:ListTagsForResource",
      "ecr:TagResource",
      "ecr:UntagResource",
    ]
    resources = [aws_ecr_repository.worker.arn]
  }

  # ------------------------------------------------------------------
  # S3 — Terraform remote state backend
  # ------------------------------------------------------------------
  statement {
    sid    = "TerraformStateS3"
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
    ]
    resources = ["arn:aws:s3:::ipp-tfstate-marcogerstmann/*"]
  }

  statement {
    sid       = "TerraformStateS3List"
    effect    = "Allow"
    actions   = ["s3:ListBucket"]
    resources = ["arn:aws:s3:::ipp-tfstate-marcogerstmann"]
  }

  # ------------------------------------------------------------------
  # DynamoDB — Terraform state locking
  # ------------------------------------------------------------------
  statement {
    sid    = "TerraformStateLock"
    effect = "Allow"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:DeleteItem",
    ]
    resources = ["arn:aws:dynamodb:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:table/terraform-locks"]
  }

  # ------------------------------------------------------------------
  # Lambda — Terraform creates/updates both the ingest and worker functions
  # ------------------------------------------------------------------
  statement {
    sid    = "LambdaManage"
    effect = "Allow"
    actions = [
      "lambda:CreateFunction",
      "lambda:DeleteFunction",
      "lambda:GetFunction",
      "lambda:GetFunctionConfiguration",
      "lambda:UpdateFunctionCode",
      "lambda:UpdateFunctionConfiguration",
      "lambda:AddPermission",
      "lambda:RemovePermission",
      "lambda:GetPolicy",
      "lambda:GetFunctionCodeSigningConfig",
      "lambda:ListTags",
      "lambda:ListVersionsByFunction",
      "lambda:TagResource",
      "lambda:UntagResource",
    ]
    resources = [
      "arn:aws:lambda:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:function:ipp-*",
    ]
  }

  # Event source mappings use a separate ARN format (event-source-mapping:<uuid>)
  # that doesn't match the function ARN pattern above.
  statement {
    sid    = "LambdaESMManage"
    effect = "Allow"
    actions = [
      "lambda:CreateEventSourceMapping",
      "lambda:UpdateEventSourceMapping",
      "lambda:DeleteEventSourceMapping",
      "lambda:GetEventSourceMapping",
      "lambda:ListEventSourceMappings",
      "lambda:ListTags",
      "lambda:TagResource",
      "lambda:UntagResource",
    ]
    resources = [
      "arn:aws:lambda:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:event-source-mapping:*",
    ]
  }

  # ------------------------------------------------------------------
  # IAM — Terraform manages execution roles for the Lambda functions.
  # Scoped to the project prefix so this role cannot touch unrelated roles.
  # ------------------------------------------------------------------
  statement {
    sid    = "IAMManage"
    effect = "Allow"
    actions = [
      "iam:CreateRole",
      "iam:DeleteRole",
      "iam:GetRole",
      "iam:PassRole",
      "iam:AttachRolePolicy",
      "iam:DetachRolePolicy",
      "iam:PutRolePolicy",
      "iam:DeleteRolePolicy",
      "iam:GetRolePolicy",
      "iam:ListRolePolicies",
      "iam:ListAttachedRolePolicies",
      "iam:TagRole",
      "iam:UntagRole",
      "iam:ListRoleTags",
    ]
    resources = [
      "arn:aws:iam::${data.aws_caller_identity.current.account_id}:role/ipp-*",
    ]
  }

  statement {
    sid    = "IAMPolicyManage"
    effect = "Allow"
    actions = [
      "iam:CreatePolicy",
      "iam:DeletePolicy",
      "iam:GetPolicy",
      "iam:GetPolicyVersion",
      "iam:ListPolicyVersions",
      "iam:CreatePolicyVersion",
      "iam:DeletePolicyVersion",
      "iam:ListEntitiesForPolicy",
      "iam:TagPolicy",
      "iam:UntagPolicy",
    ]
    resources = [
      "arn:aws:iam::${data.aws_caller_identity.current.account_id}:policy/ipp-*",
    ]
  }

  statement {
    sid    = "IAMOIDCManage"
    effect = "Allow"
    actions = [
      "iam:CreateOpenIDConnectProvider",
      "iam:DeleteOpenIDConnectProvider",
      "iam:GetOpenIDConnectProvider",
      "iam:UpdateOpenIDConnectProviderThumbprint",
      "iam:TagOpenIDConnectProvider",
    ]
    resources = [aws_iam_openid_connect_provider.github.arn]
  }

  # ------------------------------------------------------------------
  # SQS — Terraform manages the ingest events queue
  # ------------------------------------------------------------------
  statement {
    sid    = "SQSManage"
    effect = "Allow"
    actions = [
      "sqs:CreateQueue",
      "sqs:DeleteQueue",
      "sqs:GetQueueAttributes",
      "sqs:SetQueueAttributes",
      "sqs:GetQueueUrl",
      "sqs:ListQueueTags",
      "sqs:TagQueue",
      "sqs:UntagQueue",
    ]
    resources = [
      "arn:aws:sqs:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:ipp-*",
    ]
  }

  # ------------------------------------------------------------------
  # API Gateway — Terraform manages the HTTP API and its routes
  # ------------------------------------------------------------------
  statement {
    sid       = "APIGatewayManage"
    effect    = "Allow"
    actions   = ["apigateway:*"]
    # API Gateway ARNs are structured around /apis/<id>/... — scoping to a
    # specific API ID isn't possible until after first apply, so we use * here.
    resources = ["arn:aws:apigateway:${data.aws_region.current.id}::/*"]
  }

  # ------------------------------------------------------------------
  # DynamoDB — Terraform manages the insights table
  # ------------------------------------------------------------------
  statement {
    sid    = "DynamoDBManage"
    effect = "Allow"
    actions = [
      "dynamodb:CreateTable",
      "dynamodb:DeleteTable",
      "dynamodb:DescribeTable",
      "dynamodb:UpdateTable",
      "dynamodb:ListTagsOfResource",
      "dynamodb:TagResource",
      "dynamodb:UntagResource",
      "dynamodb:DescribeContinuousBackups",
      "dynamodb:DescribeTimeToLive",
    ]
    resources = [
      "arn:aws:dynamodb:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:table/ipp-*",
    ]
  }

  # ------------------------------------------------------------------
  # CloudWatch Logs — Lambda execution roles emit logs; Terraform may
  # manage log group retention settings via the Lambda modules.
  # ------------------------------------------------------------------
  statement {
    sid    = "LogsManage"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:DeleteLogGroup",
      "logs:PutRetentionPolicy",
      "logs:DeleteRetentionPolicy",
      "logs:ListTagsLogGroup",
      "logs:ListTagsForResource",
      "logs:TagLogGroup",
      "logs:TagResource",
      "logs:UntagLogGroup",
      "logs:UntagResource",
    ]
    resources = [
      "arn:aws:logs:${data.aws_region.current.id}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/ipp-*",
    ]
  }

  # DescribeLogGroups is called with a prefix filter by the Terraform AWS provider,
  # which AWS resolves to a root resource ARN — the log group prefix scope above
  # doesn't cover it, so it requires * here.
  statement {
    sid       = "LogsDescribe"
    effect    = "Allow"
    actions   = ["logs:DescribeLogGroups"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "github_actions" {
  name   = "ipp-github-actions-policy"
  role   = aws_iam_role.github_actions.name
  policy = data.aws_iam_policy_document.github_actions_permissions.json
}

output "github_actions_role_arn" {
  description = "ARN to set as the AWS_ROLE_ARN GitHub Actions secret"
  value       = aws_iam_role.github_actions.arn
}
