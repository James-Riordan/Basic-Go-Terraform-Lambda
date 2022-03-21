resource "aws_iam_role_policy" "lambda_app_policy" {
    name = "lambda_app_policy"
    role = "aws_iam_role.lambda_app_role.id"
    policy = file("../custom_policy_documents/IAM/lambda_app_policy.json")
}

resource "aws_iam_role" "lambda_app_role" {
    name = "lambda_app_role"
    assume_role_policy = file("../custom_policy_documents/IAM/lambda_assume_role_policy.json")
}
