package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-lambda-go/events"
)

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("method=%s path=%s requestId=%s", req.RequestContext.HTTP.Method, req.RawPath, req.RequestContext.RequestID)
	log.Printf("headers=%v", req.Headers)
	fmt.Println("body=", req.Body)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: `{"ok":true}`,
	}, nil
}
