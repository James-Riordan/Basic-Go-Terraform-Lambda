// contains the env variables used in Terraform

package handler

import (
	"os"
)

type config struct {
	lambdaName string
}

// NewConfigFromEnv -
func NewConfigFromEnv() *config {

	return &config{
		lambdaName: os.Getenv("LAMBDA_NAME"),
	}
}
