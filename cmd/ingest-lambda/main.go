package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mgerstmannsf/insight-processing-platform/internal/adapters/inbound/lambda/ingest"
)

func main() {
	lambda.Start(ingest.Handler)
}
