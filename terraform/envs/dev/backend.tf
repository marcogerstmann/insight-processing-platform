// TODO: Currently not configured. It's recommended to configure a remote backend (S3 + DynamoDB) and remove local with *.tfstate.

// terraform {
//   backend "s3" {
//     bucket         = "my-terraform-state-bucket"
//     key            = "insight-processing-platform/dev/terraform.tfstate"
//     region         = "eu-central-1"
//     dynamodb_table = "terraform-locks"
//   }
// }
