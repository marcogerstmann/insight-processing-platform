#!/usr/bin/env bash
set -euo pipefail

# Bootstraps the Terraform S3 remote backend: state bucket + DynamoDB lock table.
# Idempotent — safe to re-run (e.g. when setting up a new machine).
#
# IMPORTANT: BUCKET must match the `bucket` value in terraform/envs/dev/backend.tf.
# The bucket name is deliberately account-ID-free; S3 names are globally unique on
# their own. If this name is already taken globally, change it here AND in backend.tf.

BUCKET="${TF_STATE_BUCKET:-ipp-tfstate-marcogerstmann}"
LOCK_TABLE="${TF_LOCK_TABLE:-terraform-locks}"
REGION="${AWS_REGION:-eu-central-1}"

echo "==> Backend bootstrap: bucket=$BUCKET lock_table=$LOCK_TABLE region=$REGION"

# --- S3 state bucket ---
if aws s3api head-bucket --bucket "$BUCKET" >/dev/null 2>&1; then
  echo "    bucket already exists, skipping create"
else
  echo "    creating bucket"
  aws s3api create-bucket \
    --bucket "$BUCKET" \
    --region "$REGION" \
    --create-bucket-configuration LocationConstraint="$REGION"
fi

echo "    enabling versioning"
aws s3api put-bucket-versioning \
  --bucket "$BUCKET" \
  --versioning-configuration Status=Enabled

echo "    enabling default encryption (AES256)"
aws s3api put-bucket-encryption \
  --bucket "$BUCKET" \
  --server-side-encryption-configuration \
  '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'

echo "    blocking all public access"
aws s3api put-public-access-block \
  --bucket "$BUCKET" \
  --public-access-block-configuration \
  BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true

# --- DynamoDB lock table ---
if aws dynamodb describe-table --table-name "$LOCK_TABLE" --region "$REGION" >/dev/null 2>&1; then
  echo "    lock table already exists, skipping create"
else
  echo "    creating lock table"
  aws dynamodb create-table \
    --table-name "$LOCK_TABLE" \
    --attribute-definitions AttributeName=LockID,AttributeType=S \
    --key-schema AttributeName=LockID,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region "$REGION" >/dev/null
  echo "    waiting for lock table to become ACTIVE"
  aws dynamodb wait table-exists --table-name "$LOCK_TABLE" --region "$REGION"
fi

echo "==> Backend bootstrap complete."
