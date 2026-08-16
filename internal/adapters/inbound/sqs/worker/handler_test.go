package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/marcogerstmann/insight-processing-platform/internal/apperr"
	"github.com/marcogerstmann/insight-processing-platform/internal/application/insight"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

type spyService struct {
	processed []domain.Insight
	errByID   map[string]error
}

func (s *spyService) Process(_ context.Context, i domain.Insight) (insight.Result, error) {
	s.processed = append(s.processed, i)
	if err, ok := s.errByID[i.ID]; ok {
		return insight.Result{}, err
	}
	return insight.Result{Inserted: true}, nil
}

func (s *spyService) ListByTenantID(_ context.Context, _, _ string) ([]domain.Insight, error) {
	return nil, nil
}

func (s *spyService) ListTags(_ context.Context, _ string) ([]domain.TagSummary, error) {
	return nil, nil
}

type spyDLQ struct {
	sentIDs []string
	reasons []error
	sendErr error
}

func (s *spyDLQ) Send(_ context.Context, rec events.SQSMessage, reason error) error {
	s.sentIDs = append(s.sentIDs, rec.MessageId)
	s.reasons = append(s.reasons, reason)
	return s.sendErr
}

func attr(v string) events.SQSMessageAttribute {
	return events.SQSMessageAttribute{DataType: "String", StringValue: &v}
}

// validBody marshals the package's own DTO so the fixture cannot drift from
// the wire format the mapper actually parses.
func validBody(t *testing.T, highlightID string) string {
	t.Helper()
	b, err := json.Marshal(messageDTO{
		Source:    "readwise",
		EventType: "highlight.created",
		Highlight: highlightDTO{ID: highlightID, Text: "  hello world  "},
	})
	if err != nil {
		t.Fatalf("marshal fixture body: %v", err)
	}
	return string(b)
}

func record(messageID, idempotencyKey, body string) events.SQSMessage {
	return events.SQSMessage{
		MessageId: messageID,
		Body:      body,
		MessageAttributes: map[string]events.SQSMessageAttribute{
			attrTenantID:       attr("t-1"),
			attrIdempotencyKey: attr(idempotencyKey),
		},
	}
}

func TestHandler_Handle_ValidRecord_ProcessesAndSkipsDLQ(t *testing.T) {
	svc := &spyService{}
	dlq := &spyDLQ{}
	h := NewHandler(svc, dlq)

	err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{record("m-1", "idk-1", validBody(t, "hl-1"))},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(svc.processed) != 1 {
		t.Fatalf("expected 1 processed insight, got %d", len(svc.processed))
	}
	got := svc.processed[0]
	// The idempotency key from the message attribute becomes the insight ID —
	// that is what makes an SQS redelivery a no-op downstream (ADR-008).
	if got.ID != "idk-1" {
		t.Fatalf("expected insight ID to be the idempotency key %q, got %q", "idk-1", got.ID)
	}
	if got.TenantID != "t-1" {
		t.Fatalf("expected tenant %q, got %q", "t-1", got.TenantID)
	}
	if got.Text != "hello world" {
		t.Fatalf("expected trimmed text %q, got %q", "hello world", got.Text)
	}
	if len(dlq.sentIDs) != 0 {
		t.Fatalf("expected nothing routed to DLQ, got %v", dlq.sentIDs)
	}
}

func TestHandler_Handle_PermanentMappingError_RoutesToDLQ_NoRetry(t *testing.T) {
	cases := map[string]events.SQSMessage{
		"malformed json body": record("m-json", "idk-json", "{not json"),
		"missing highlight id": record("m-hlid", "idk-hlid",
			`{"source":"readwise","event_type":"highlight.created","highlight":{"text":"x"}}`),
		"missing source": record("m-src", "idk-src",
			`{"event_type":"highlight.created","highlight":{"id":"hl-1","text":"x"}}`),
		"missing idempotency key attribute": {
			MessageId: "m-attr",
			Body:      validBody(t, "hl-1"),
			MessageAttributes: map[string]events.SQSMessageAttribute{
				attrTenantID: attr("t-1"),
			},
		},
		"tenant drift between body and attribute": {
			MessageId: "m-drift",
			Body:      `{"tenant_id":"t-999","source":"readwise","event_type":"highlight.created","highlight":{"id":"hl-1","text":"x"}}`,
			MessageAttributes: map[string]events.SQSMessageAttribute{
				attrTenantID:       attr("t-1"),
				attrIdempotencyKey: attr("idk-drift"),
			},
		},
	}

	for name, rec := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &spyService{}
			dlq := &spyDLQ{}
			h := NewHandler(svc, dlq)

			// No error returned: a poison message must not trigger redelivery,
			// it is unfixable and burns the retry budget for nothing (ADR-009).
			if err := h.Handle(context.Background(), events.SQSEvent{Records: []events.SQSMessage{rec}}); err != nil {
				t.Fatalf("expected no error so SQS deletes the message, got %v", err)
			}
			if len(svc.processed) != 0 {
				t.Fatalf("expected the service never to see a malformed record, got %v", svc.processed)
			}
			if len(dlq.sentIDs) != 1 || dlq.sentIDs[0] != rec.MessageId {
				t.Fatalf("expected %q routed to DLQ, got %v", rec.MessageId, dlq.sentIDs)
			}
			if !errors.As(dlq.reasons[0], &apperr.PermanentError{}) {
				t.Fatalf("expected a PermanentError as the DLQ reason, got %v", dlq.reasons[0])
			}
		})
	}
}

func TestHandler_Handle_PermanentServiceError_RoutesToDLQ_NoRetry(t *testing.T) {
	permanent := apperr.PermanentError{Err: errors.New("missing id")}
	svc := &spyService{errByID: map[string]error{"idk-1": permanent}}
	dlq := &spyDLQ{}
	h := NewHandler(svc, dlq)

	err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{record("m-1", "idk-1", validBody(t, "hl-1"))},
	})
	if err != nil {
		t.Fatalf("expected no error for a permanent failure, got %v", err)
	}
	if len(dlq.sentIDs) != 1 || dlq.sentIDs[0] != "m-1" {
		t.Fatalf("expected m-1 routed to DLQ, got %v", dlq.sentIDs)
	}
	if !errors.Is(dlq.reasons[0], permanent.Err) {
		t.Fatalf("expected the original cause attached to the DLQ record, got %v", dlq.reasons[0])
	}
}

func TestHandler_Handle_TransientServiceError_ReturnsError_SkipsDLQ(t *testing.T) {
	transient := errors.New("dynamodb throttled")
	svc := &spyService{errByID: map[string]error{"idk-1": transient}}
	dlq := &spyDLQ{}
	h := NewHandler(svc, dlq)

	err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{record("m-1", "idk-1", validBody(t, "hl-1"))},
	})
	// Returning the error is what preserves at-least-once delivery: SQS keeps
	// the message invisible and redelivers it.
	if !errors.Is(err, transient) {
		t.Fatalf("expected the transient error returned so SQS redelivers, got %v", err)
	}
	if len(dlq.sentIDs) != 0 {
		t.Fatalf("expected a retryable failure never to reach the DLQ, got %v", dlq.sentIDs)
	}
}

func TestHandler_Handle_PoisonRecordDoesNotBlockTheRestOfTheBatch(t *testing.T) {
	svc := &spyService{}
	dlq := &spyDLQ{}
	h := NewHandler(svc, dlq)

	err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{
			record("m-bad", "idk-bad", "{not json"),
			record("m-good", "idk-good", validBody(t, "hl-2")),
		},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(svc.processed) != 1 || svc.processed[0].ID != "idk-good" {
		t.Fatalf("expected the healthy record processed after the poison one, got %v", svc.processed)
	}
	if len(dlq.sentIDs) != 1 || dlq.sentIDs[0] != "m-bad" {
		t.Fatalf("expected only m-bad in the DLQ, got %v", dlq.sentIDs)
	}
}

func TestHandler_Handle_DLQSendFailure_IsSwallowedAsDegradedPath(t *testing.T) {
	svc := &spyService{}
	dlq := &spyDLQ{sendErr: errors.New("sqs unavailable")}
	h := NewHandler(svc, dlq)

	// ADR-009: a failed DLQ send is logged and dropped; the record then falls
	// back to ordinary redrive rather than failing the batch.
	err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{record("m-bad", "idk-bad", "{not json")},
	})
	if err != nil {
		t.Fatalf("expected a failed DLQ send to be swallowed, got %v", err)
	}
	if len(dlq.sentIDs) != 1 {
		t.Fatalf("expected one DLQ attempt, got %v", dlq.sentIDs)
	}
}

// A transient failure fails the whole batch, so records after it are never
// reached and records before it are reprocessed on redelivery. That is only
// safe because batch_size = 1 (ADR-007/ADR-009); this test fails the day
// someone raises it without switching to ReportBatchItemFailures.
func TestHandler_Handle_TransientError_AbortsRemainderOfBatch(t *testing.T) {
	transient := errors.New("dynamodb throttled")
	svc := &spyService{errByID: map[string]error{"idk-2": transient}}
	dlq := &spyDLQ{}
	h := NewHandler(svc, dlq)

	err := h.Handle(context.Background(), events.SQSEvent{
		Records: []events.SQSMessage{
			record("m-1", "idk-1", validBody(t, "hl-1")),
			record("m-2", "idk-2", validBody(t, "hl-2")),
			record("m-3", "idk-3", validBody(t, "hl-3")),
		},
	})
	if !errors.Is(err, transient) {
		t.Fatalf("expected the transient error returned, got %v", err)
	}

	if len(svc.processed) != 2 {
		t.Fatalf("expected the batch to abort at the failing record, got %v", svc.processed)
	}
	if svc.processed[1].ID != "idk-2" {
		t.Fatalf("expected idk-2 to be the last attempted record, got %q", svc.processed[1].ID)
	}
}
