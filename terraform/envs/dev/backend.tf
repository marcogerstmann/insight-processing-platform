# Remote state backend (S3) with DynamoDB-based state locking.
# Bootstrap the bucket + lock table once with `make tf-backend-bootstrap`.
#
# The bucket name is intentionally account-ID-free (S3 names are globally unique
# on their own). Backend blocks cannot use variables, so the value is a literal.
# Keep it in sync with terraform/scripts/bootstrap-backend.sh.

terraform {
  backend "s3" {
    bucket       = "ipp-tfstate-marcogerstmann"
    key          = "insight-processing-platform/dev/terraform.tfstate"
    region       = "eu-central-1"
    use_lockfile = true
    encrypt      = true
  }
}
