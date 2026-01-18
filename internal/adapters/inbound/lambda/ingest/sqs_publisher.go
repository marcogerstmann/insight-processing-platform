package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSPublisher struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSPublisher(ctx context.Context) (*SQSPublisher, error) {
	queueURL := os.Getenv("INGEST_QUEUE_URL")
	if queueURL == "" {
		return nil, errors.New("missing env INGEST_QUEUE_URL")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &SQSPublisher{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}, nil
}

func (p *SQSPublisher) Publish(
	ctx context.Context,
	ev IngestEvent,
	idempotencyKey string,
	receivedAt time.Time,
) error {

	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(p.queueURL),
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"tenant_id": {
				DataType:    aws.String("String"),
				StringValue: aws.String(ev.TenantID),
			},
			"idempotency_key": {
				DataType:    aws.String("String"),
				StringValue: aws.String(idempotencyKey),
			},
			"event_type": {
				DataType:    aws.String("String"),
				StringValue: aws.String(ev.EventType),
			},
			"received_at": {
				DataType:    aws.String("String"),
				StringValue: aws.String(receivedAt.UTC().Format(time.RFC3339)),
			},
		},
	})

	return err
}
