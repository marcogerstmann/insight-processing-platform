package event

import "context"

type PublishMessage struct {
	Body       []byte
	Attributes map[string]string
}

type EventPublisher interface {
	Publish(ctx context.Context, msg PublishMessage) error
}
