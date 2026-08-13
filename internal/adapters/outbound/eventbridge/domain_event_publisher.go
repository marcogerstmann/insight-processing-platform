package eventbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
	"github.com/marcogerstmann/insight-processing-platform/internal/ports"
)

// eventSource identifies IPP as the publisher on the bus - subscriber rules
// pattern-match on this alongside DetailType.
const eventSource = "ipp.core"

type DomainEventPublisher struct {
	client  *eventbridge.Client
	busName string
}

var _ ports.DomainEventPublisher = (*DomainEventPublisher)(nil)

func NewDomainEventPublisher(ctx context.Context) (*DomainEventPublisher, error) {
	busName := os.Getenv("DOMAIN_EVENTS_BUS_NAME")
	if busName == "" {
		return nil, errors.New("missing env DOMAIN_EVENTS_BUS_NAME")
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &DomainEventPublisher{
		client:  eventbridge.NewFromConfig(cfg),
		busName: busName,
	}, nil
}

func (p *DomainEventPublisher) Publish(ctx context.Context, event domain.DomainEvent) error {
	detail, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal domain event: %w", err)
	}

	out, err := p.client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []types.PutEventsRequestEntry{
			{
				EventBusName: aws.String(p.busName),
				Source:       aws.String(eventSource),
				DetailType:   aws.String(string(event.EventType)),
				Detail:       aws.String(string(detail)),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("put event to eventbridge: %w", err)
	}

	// PutEvents returns 200 even when individual entries fail; a non-zero
	// FailedEntryCount must be surfaced explicitly or it gets swallowed.
	if out.FailedEntryCount > 0 {
		reason := "unknown reason"
		if len(out.Entries) > 0 && out.Entries[0].ErrorMessage != nil {
			reason = *out.Entries[0].ErrorMessage
		}
		return fmt.Errorf("put event to eventbridge failed: %s", reason)
	}

	return nil
}
