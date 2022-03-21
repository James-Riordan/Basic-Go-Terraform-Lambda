package main

import (
	"github.com/James-Riordan/Basic-Go-Terraform-Lambda/lambda_app/handler"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	handler := handler.Create()
	lambda.Start(handler.Run)
}
