module "dynamodb_insights" {
  source = "../../modules/dynamodb"

  name = "${var.project}-insights"

  enable_tag_gsi = true

  tags = {
    Project = var.project
    Env     = var.env
  }
}
