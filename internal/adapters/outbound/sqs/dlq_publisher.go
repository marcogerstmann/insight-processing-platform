package sqs

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSDLQPublisher struct {
	client *sqs.Client
	dlqURL string
}

func NewSQSDLQPublisher(ctx context.Context) (*SQSDLQPublisher, error) {
	dlqURL := os.Getenv("INGEST_DLQ_URL")
	if dlqURL == "" {
		return nil, errors.New("missing env INGEST_DLQ_URL")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &SQSDLQPublisher{
		client: sqs.NewFromConfig(cfg),
		dlqURL: dlqURL,
	}, nil
}

func (p *SQSDLQPublisher) Send(ctx context.Context, record events.SQSMessage, reason error) error {
	attrs := make(map[string]types.MessageAttributeValue, len(record.MessageAttributes)+1)
	for k, v := range record.MessageAttributes {
		attrs[k] = types.MessageAttributeValue{
			DataType:    aws.String(v.DataType),
			StringValue: v.StringValue,
			BinaryValue: v.BinaryValue,
		}
	}
	attrs["failure_reason"] = types.MessageAttributeValue{
		DataType:    aws.String("String"),
		StringValue: aws.String(reason.Error()),
	}

	_, err := p.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(p.dlqURL),
		MessageBody:       aws.String(record.Body),
		MessageAttributes: attrs,
	})
	return err
}
