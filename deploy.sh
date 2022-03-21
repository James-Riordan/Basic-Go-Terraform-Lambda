#!/bin/bash

echo -e "\nStarting deployment\n"

tfswitch 1.0.0

rm -rf ./bin

echo "Building Go Binary"

cd lambda_app/handler
go test ./...
env GOOS=linux GOARCH=amd64 go build -o ../../bin/lambda
cd ../..

echo "lambda_app module"
cd infrastructure
terraform init -input=false
terraform apply -input=false -auto-approve
cd ../

echo -e "\nDeployment done\n"