package sqs

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	port "github.com/mgerstmannsf/insight-processing-platform/internal/ports/outbound"
)

type Publisher struct {
	client   *sqs.Client
	queueURL string
}

func NewPublisher(ctx context.Context) (*Publisher, error) {
	queueURL := os.Getenv("INGEST_QUEUE_URL")
	if queueURL == "" {
		return nil, errors.New("missing env INGEST_QUEUE_URL")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, msg port.PublishMessage) error {
	attrs := make(map[string]types.MessageAttributeValue, len(msg.Attributes))
	for k, v := range msg.Attributes {
		attrs[k] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(v),
		}
	}

	_, err := p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(p.queueURL),
		MessageBody:       aws.String(string(msg.Body)),
		MessageAttributes: attrs,
	})
	return err
}
