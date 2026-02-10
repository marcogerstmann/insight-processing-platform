package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/marcogerstmann/insight-processing-platform/internal/domain"
)

const (
	attrTenantID       = "tenant_id"
	attrIdempotencyKey = "idempotency_key"
)

func MapRecordToDomain(rec events.SQSMessage) (domain.IngestEvent, error) {
	dto, err := parseBody(rec.Body)
	if err != nil {
		return domain.IngestEvent{}, err
	}

	// Message body
	ev, err := MapMessageDTOToDomain(dto)
	if err != nil {
		return domain.IngestEvent{}, err
	}

	// Message attributes
	tenantID, err := getRequiredAttr(rec, attrTenantID)
	if err != nil {
		return domain.IngestEvent{}, err
	}
	idk, err := getRequiredAttr(rec, attrIdempotencyKey)
	if err != nil {
		return domain.IngestEvent{}, err
	}

	// Drift checks between message body and attributes
	if strings.TrimSpace(dto.TenantID) != "" && dto.TenantID != tenantID {
		return domain.IngestEvent{}, PermanentError{Err: fmt.Errorf("tenant_id mismatch: body=%q attr=%q", dto.TenantID, tenantID)}
	}
	if strings.TrimSpace(dto.IdempotencyKey) != "" && dto.IdempotencyKey != idk {
		return domain.IngestEvent{}, PermanentError{Err: fmt.Errorf("idempotency_key mismatch: body=%q attr=%q", dto.IdempotencyKey, idk)}
	}

	ev.TenantID = tenantID
	ev.IdempotencyKey = idk

	return ev, nil
}

func getRequiredAttr(rec events.SQSMessage, key string) (string, error) {
	a, ok := rec.MessageAttributes[key]
	if !ok || a.StringValue == nil {
		return "", PermanentError{Err: fmt.Errorf("missing message attribute %s", key)}
	}
	v := strings.TrimSpace(*a.StringValue)
	if v == "" {
		return "", PermanentError{Err: fmt.Errorf("empty message attribute %s", key)}
	}
	return v, nil
}

func parseBody(body string) (MessageDTO, error) {
	var dto MessageDTO
	if err := json.Unmarshal([]byte(body), &dto); err != nil {
		return MessageDTO{}, PermanentError{Err: fmt.Errorf("invalid json body: %w", err)}
	}
	return dto, nil
}
