resource "aws_iam_role" "this" {
  name = var.name

  assume_role_policy = var.assume_role_policy
}

resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.this.name
  policy_arn = var.basic_execution_policy_arn
}
