package ports

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

type DLQPublisher interface {
	Send(ctx context.Context, record events.SQSMessage, reason error) error
}
