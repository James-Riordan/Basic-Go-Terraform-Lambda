resource "aws_lambda_function" "lambda_app" {
    function_name = "lambda_app_1"
    image_uri     = "${aws_ecr_repository.image_storage.repository_url}:latest"
    package_type  = "Image"
    role          = aws_iam_role.lambda_app_role.arn
}

