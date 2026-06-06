module "dynamodb_insights" {
  source = "../../modules/dynamodb"

  name = "${var.project}-insights"

  tags = {
    Project = var.project
    Env     = var.env
  }
}
